#!/bin/bash

DOMAIN="yourdomain.com"
APP_USERNAME="yourusername"
DEST_DIR="/home/$APP_USERNAME/certs"

cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem $DEST_DIR
cp /etc/letsencrypt/live/$DOMAIN/privkey.pem $DEST_DIR
chown $APP_USERNAME:$APP_USERNAME $DEST_DIR/{fullchain.pem,privkey.pem}
chmod 600 $DEST_DIR/{fullchain.pem,privkey.pem}