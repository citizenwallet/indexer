#!/bin/bash

DOMAIN="yourdomain.com"

chmod 644 /etc/letsencrypt/live/$DOMAIN/{fullchain.pem,privkey.pem}