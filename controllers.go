package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func jsonError(c *gin.Context, code int, msg string) {
	c.JSON(code, gin.H{"error": msg})
}

func getUserIDFromContext(c *gin.Context) (uint, bool) {
	uid, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}

	switch v := uid.(type) {
	case uint:
		return v, true
	case int:
		return uint(v), true
	case float64:
		return uint(v), true
	default:
		_ = v
		return 0, false
	}
}

type CreateEventRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Location    string `json:"location"`
	Date        string `json:"date" binding:"required"` // expect ISO8601 or "YYYY-MM-DD"
}

func CreateEvent(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body CreateEventRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	var eventDate time.Time
	var err error
	eventDate, err = time.Parse(time.RFC3339, body.Date)
	if err != nil {
		eventDate, err = time.Parse("2006-01-02", body.Date)
		if err != nil {
			jsonError(c, http.StatusBadRequest, "invalid date format (use RFC3339 or YYYY-MM-DD)")
			return
		}
	}
	now := time.Now()

	if !eventDate.After(now) {
		jsonError(c, http.StatusBadRequest, "event date must be in the future")
		return
	}

	ev := Event{
		Title:       strings.TrimSpace(body.Title),
		Description: body.Description,
		Location:    body.Location,
		Date:        eventDate,
		OrganizerID: userID,
	}

	if err := DB.Create(&ev).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "could not create event: "+err.Error())
		return
	}

	org := EventAttendee{
		EventID: ev.ID,
		UserID:  userID,
		Role:    "organizer",
		Status:  "",
	}

	_ = DB.Where("event_id = ? AND user_id = ?", ev.ID, userID).FirstOrCreate(&org)

	c.JSON(http.StatusCreated, ev)
}

func GetOrganizedEvents(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var events []Event
	if err := DB.Preload("Tasks").
		Joins("LEFT JOIN event_attendees ea ON ea.event_id = events.id").
		Where("events.organizer_id = ? OR (ea.user_id = ? AND ea.role = ?)", userID, userID, "organizer").
		Group("events.id").
		Order("events.date asc").
		Find(&events).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, events)
}

func GetInvitedEvents(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var attendances []EventAttendee
	if err := DB.Where("user_id = ? AND role IN ?", userID, []string{"attendee", "organizer"}).Find(&attendances).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	if len(attendances) == 0 {
		c.JSON(http.StatusOK, []Event{})
		return
	}

	ids := make([]uint, 0, len(attendances))
	for _, a := range attendances {
		ids = append(ids, a.EventID)
	}

	var events []Event
	if err := DB.Preload("Tasks").Where("id IN ?", ids).Order("date asc").Find(&events).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, events)
}

func DeleteEvent(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	idParam := c.Param("id")
	if idParam == "" {
		jsonError(c, http.StatusBadRequest, "missing event id")
		return
	}
	id, err := strconv.Atoi(idParam)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid event id")
		return
	}

	var ev Event
	if err := DB.First(&ev, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			jsonError(c, http.StatusNotFound, "event not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	if ev.OrganizerID != userID {
		jsonError(c, http.StatusForbidden, "only organizer can delete the event")
		return
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("event_id = ?", ev.ID).Delete(&EventAttendee{}).Error; err != nil {
			return err
		}
		if err := tx.Where("event_id = ?", ev.ID).Delete(&Task{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&Event{}, ev.ID).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		jsonError(c, http.StatusInternalServerError, "delete failed: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "event deleted"})
}

type InviteRequest struct {
	UserID uint   `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"` // "attendee" or "organizer"
}

func InviteUser(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// parse event id
	eventID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id"})
		return
	}
	eventID := uint(eventID64)

	// bind request
	var body InviteRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body: " + err.Error()})
		return
	}

	role := strings.ToLower(body.Role)
	if role != "attendee" && role != "organizer" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role must be attendee or organizer"})
		return
	}

	// event exists?
	var ev Event
	if err := DB.First(&ev, eventID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}

	// permission: only organizer can invite
	var inviterAtt EventAttendee
	inviterIsOrganizer := (ev.OrganizerID == userID)

	if !inviterIsOrganizer {
		// Maybe they were added as organizer previously
		err := DB.Where("event_id = ? AND user_id = ? AND role = ?", eventID, userID, "organizer").
			First(&inviterAtt).Error
		if err == nil {
			inviterIsOrganizer = true
		}
	}

	if !inviterIsOrganizer {
		c.JSON(http.StatusForbidden, gin.H{"error": "only organizers can invite"})
		return
	}

	// check invitee exists
	var invitee User
	if err := DB.First(&invitee, body.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "invited user not found"})
		return
	}

	// check if already invited
	var existing EventAttendee
	if err := DB.Where("event_id = ? AND user_id = ?", eventID, invitee.ID).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"message": "user already a participant"})
		return
	}

	// create attendee
	newAtt := EventAttendee{
		EventID: eventID,
		UserID:  invitee.ID,
		Role:    role,
		Status:  "",
	}

	if err := DB.Create(&newAtt).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create invitation: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User invited successfully",
		"user_id": invitee.ID,
		"role":    role,
	})
}

type AttendanceRequest struct {
	Status string `json:"status" binding:"required"`
}

func SetAttendance(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	idParam := c.Param("id")
	eventID64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	eventID := uint(eventID64)

	var body AttendanceRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	normalized := strings.Title(strings.ToLower(strings.TrimSpace(body.Status)))
	if normalized != "Going" && normalized != "Maybe" && normalized != "Not Going" {
		jsonError(c, http.StatusBadRequest, "status must be one of: Going, Maybe, Not Going")
		return
	}

	var ev Event
	if err := DB.First(&ev, eventID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			jsonError(c, http.StatusNotFound, "event not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	var att EventAttendee
	if err := DB.Where("event_id = ? AND user_id = ?", eventID, userID).First(&att).Error; err != nil {
		if err == gorm.ErrRecordNotFound {

			att = EventAttendee{
				EventID: eventID,
				UserID:  userID,
				Role:    "attendee",
				Status:  normalized,
			}
			if err := DB.Create(&att).Error; err != nil {
				jsonError(c, http.StatusInternalServerError, "could not set attendance: "+err.Error())
				return
			}
			c.JSON(http.StatusOK, att)
			return
		}
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	att.Status = normalized
	if err := DB.Save(&att).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "could not update status: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, att)
}

func GetEventAttendees(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	idParam := c.Param("id")
	eventID64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	eventID := uint(eventID64)

	var ev Event
	if err := DB.First(&ev, eventID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			jsonError(c, http.StatusNotFound, "event not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	if ev.OrganizerID != userID {
		jsonError(c, http.StatusForbidden, "only organizer can view attendees")
		return
	}

	var attendees []EventAttendee
	if err := DB.Where("event_id = ?", eventID).Find(&attendees).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, attendees)
}

type CreateTaskRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
}

func CreateTask(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	idParam := c.Param("id")
	eventID64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	eventID := uint(eventID64)

	var ev Event
	if err := DB.First(&ev, eventID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			jsonError(c, http.StatusNotFound, "event not found")
			return
		}
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}
	if ev.OrganizerID != userID {
		jsonError(c, http.StatusForbidden, "only organizer can create tasks")
		return
	}

	var body CreateTaskRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	task := Task{
		EventID:     eventID,
		Title:       strings.TrimSpace(body.Title),
		Description: body.Description,
	}

	if err := DB.Create(&task).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "could not create task: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, task)
}

func GetTasksByEvent(c *gin.Context) {

	idParam := c.Param("id")
	eventID64, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid event id")
		return
	}
	eventID := uint(eventID64)

	var tasks []Task
	if err := DB.Where("event_id = ?", eventID).Find(&tasks).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, tasks)
}

type SearchRequest struct {
	Keyword   string `form:"keyword" json:"keyword"`
	StartDate string `form:"start_date" json:"start_date"`
	EndDate   string `form:"end_date" json:"end_date"`
	Role      string `form:"role" json:"role"`
	Type      string `form:"type" json:"type"`
}

func SearchHandler(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		jsonError(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req SearchRequest

	if c.Request.Method == http.MethodGet {
		if err := c.ShouldBindQuery(&req); err != nil {

			_ = err
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = err
			_ = c.ShouldBindQuery(&req)
		}
	}

	if req.Type == "" {
		req.Type = "both"
	}

	var start, end time.Time
	var err error
	if req.StartDate != "" {
		start, err = time.Parse(time.RFC3339, req.StartDate)
		if err != nil {
			start, err = time.Parse("2006-01-02", req.StartDate)
			if err != nil {
				jsonError(c, http.StatusBadRequest, "invalid start_date format")
				return
			}
		}
	}
	if req.EndDate != "" {
		end, err = time.Parse(time.RFC3339, req.EndDate)
		if err != nil {
			end, err = time.Parse("2006-01-02", req.EndDate)
			if err != nil {
				jsonError(c, http.StatusBadRequest, "invalid end_date format")
				return
			}
		}

		end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}

	keyword := strings.TrimSpace(req.Keyword)
	kw := "%" + keyword + "%"

	results := make([]interface{}, 0)

	if req.Type == "both" || req.Type == "event" {
		query := DB.Model(&Event{}).Preload("Tasks")

		if keyword != "" {
			query = query.Where("title ILIKE ? OR description ILIKE ?", kw, kw)
		}
		if !start.IsZero() {
			query = query.Where("date >= ?", start)
		}
		if !end.IsZero() {
			query = query.Where("date <= ?", end)
		}

		if req.Role != "" {
			if req.Role == "organizer" {
				query = query.Where("organizer_id = ?", userID)
			} else if req.Role == "attendee" {
				// join with attendees table
				query = query.Joins("JOIN event_attendees ea ON ea.event_id = events.id").
					Where("ea.user_id = ? AND ea.role = ?", userID, "attendee")
			} else {
				jsonError(c, http.StatusBadRequest, "role must be 'organizer' or 'attendee'")
				return
			}
		}

		var events []Event
		if err := query.Order("date asc").Find(&events).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
			return
		}
		for _, e := range events {
			results = append(results, gin.H{"type": "event", "event": e})
		}
	}

	// Search tasks (and attach event info)
	if req.Type == "both" || req.Type == "task" {
		// We'll find tasks joining with events to apply date filters and role constraints
		taskQuery := DB.Model(&Task{}).Joins("JOIN events ON events.id = tasks.event_id")

		if keyword != "" {
			// search task title/description or parent event title/description
			taskQuery = taskQuery.Where("tasks.title ILIKE ? OR tasks.description ILIKE ? OR events.title ILIKE ? OR events.description ILIKE ?", kw, kw, kw, kw)
		}
		if !start.IsZero() {
			taskQuery = taskQuery.Where("events.date >= ?", start)
		}
		if !end.IsZero() {
			taskQuery = taskQuery.Where("events.date <= ?", end)
		}
		if req.Role != "" {
			if req.Role == "organizer" {
				taskQuery = taskQuery.Where("events.organizer_id = ?", userID)
			} else if req.Role == "attendee" {
				// ensure user is attendee in event_attendees
				taskQuery = taskQuery.Joins("JOIN event_attendees ea ON ea.event_id = events.id").
					Where("ea.user_id = ? AND ea.role = ?", userID, "attendee")
			} else {
				jsonError(c, http.StatusBadRequest, "role must be 'organizer' or 'attendee'")
				return
			}
		}

		// fetch matching tasks
		var tasks []Task
		if err := taskQuery.Select("tasks.*").Order("events.date asc").Find(&tasks).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "db error: "+err.Error())
			return
		}

		// attach event data for each task
		for _, t := range tasks {
			var ev Event
			if err := DB.Where("id = ?", t.EventID).First(&ev).Error; err != nil {
				// skip if cannot find parent event
				continue
			}
			results = append(results, gin.H{"type": "task", "task": t, "event": ev})
		}
	}

	c.JSON(http.StatusOK, results)
}
