#!/bin/bash

# Đường dẫn file cấu hình nginx của dự án
CONF_SRC="./telerealm.conf"

# Đường dẫn thư mục cấu hình nginx trên server (thường là /etc/nginx/conf.d/)
CONF_DEST="/etc/nginx/conf.d/telerealm.conf"

# Copy file cấu hình vào thư mục nginx (yêu cầu quyền sudo)
echo "Copying nginx config to $CONF_DEST ..."
sudo cp "$CONF_SRC" "$CONF_DEST"

# Reload nginx để áp dụng cấu hình mới
echo "Reloading nginx ..."
sudo systemctl restart nginx

echo "Nginx config updated!"
