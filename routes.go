package main

import "github.com/gin-gonic/gin"

func SetupRoutes(r *gin.Engine) {

	// Public Routes
	r.POST("/signup", Signup)
	r.POST("/login", Login)

	// Protected Routes
	authorized := r.Group("/api")
	authorized.Use(AuthMiddleware())
	{
		// EVENTS
		authorized.POST("/events", CreateEvent)
		authorized.GET("/events/organized", GetOrganizedEvents)
		authorized.GET("/events/invited", GetInvitedEvents)
		authorized.DELETE("/events/:id", DeleteEvent)

		// INVITATIONS
		authorized.POST("/events/:id/invite", InviteUser)

		// ATTENDANCE
		authorized.POST("/events/:id/respond", SetAttendance)
		authorized.GET("/events/:id/attendees", GetEventAttendees)

		// TASKS
		authorized.POST("/events/:id/tasks", CreateTask)
		authorized.GET("/events/:id/tasks", GetTasksByEvent)

		// SEARCH
		authorized.GET("/events/search", SearchHandler)
	}
}
