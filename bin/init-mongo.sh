#!/bin/bash

# Start MongoDB service in the background
mongod --replSet "RS" --bind_ip_all &

# Wait for MongoDB to start
sleep 10

# Initiate the replica set
mongosh --eval "rs.initiate({_id: 'RS', members: [{ _id: 0, host: 'mongodb:27017' }]})"

# Create the root user
mongosh --eval "
db = db.getSiblingDB('admin');
db.createUser({
  user: 'root',
  pwd: 'example',
  roles: [{ role: 'root', db: 'admin' }]
});
"

# Keep the container running
tail -f /dev/null
