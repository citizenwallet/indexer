# Use the official Postgres 15.3 Alpine image as the base image
FROM postgres:15.3-alpine

# Set the default user and databasevar
ENV POSTGRES_USER=$POSTGRES_USER
ENV POSTGRES_PASSWORD=$POSTGRES_PASSWORD
ENV POSTGRES_DB=$POSTGRES_DB

# Copy the initial SQL script to the container
COPY ./db/init.sql /docker-entrypoint-initdb.d/
