# Use the official Postgres 15.3 Alpine image as the base image
FROM postgres:15.3-alpine

# Set the default user and database
ENV POSTGRES_USER=$POSTGRES_USER
ENV POSTGRES_PASSWORD=$POSTGRES_PASSWORD
ENV POSTGRES_DB=$POSTGRES_DB

# Copy the custom configuration file to the container
COPY db/postgresql.conf /usr/local/share/postgresql/postgresql.conf

# Copy the initial SQL script to the container
COPY db/init.sql /docker-entrypoint-initdb.d/

# Expose the default Postgres port
EXPOSE 5432

# Start Postgres
CMD ["postgres", "-c", "config_file=/usr/local/share/postgresql/postgresql.conf"]