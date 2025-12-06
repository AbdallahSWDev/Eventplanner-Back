# Use official PostgreSQL image
FROM postgres:15

# Set environment variables for database user, password, name
ENV POSTGRES_USER=eventuser
ENV POSTGRES_PASSWORD=eventpass
ENV POSTGRES_DB=eventplanner

# Expose PostgreSQL port
EXPOSE 5432
