#!/bin/bash
set -e
cd "$(dirname "$0")"
echo "Building 3 chat peers..."
advplc build chat.prw -o chat-peer-1
advplc build chat.prw -o chat-peer-2
advplc build chat.prw -o chat-peer-3
echo "Built: chat-peer-1, chat-peer-2, chat-peer-3"
chmod +x chat-peer-*
