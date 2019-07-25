#!/bin/ash

<<<<<<< HEAD
if [ -n "$UI_PORT" ]; then
    sed -i -e "s/UI_PORT/$UI_PORT/" /etc/nginx/conf.d/default.conf
else
    sed -i -e "s/UI_PORT/3000/" /etc/nginx/conf.d/default.conf
=======
if [ -n "$MF_UI_PORT" ]; then
    sed -i -e "s/MF_UI_PORT/$MF_UI_PORT/" /etc/nginx/conf.d/default.conf
else
    sed -i -e "s/MF_UI_PORT/3000/" /etc/nginx/conf.d/default.conf
>>>>>>> 649986b19faad871678d27a061e55cf4775bcac5
fi

exec nginx -g "daemon off;"
