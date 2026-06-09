#!/bin/bash

SESSION="mirrorleague"
ROOT=$(cd "$(dirname "$0")" && pwd)

tmux has-session -t $SESSION 2>/dev/null && tmux kill-session -t $SESSION

tmux new-session -d -s $SESSION

# 1: Free terminal
tmux rename-window -t $SESSION:1 "terminal"
tmux send-keys -t $SESSION:1 "cd $ROOT" Enter

# 2: Claude
tmux new-window -t $SESSION:2 -n "claude"
tmux send-keys -t $SESSION:2 "cd $ROOT && claude" Enter

# 3: Vite dev server
tmux new-window -t $SESSION:3 -n "web"
tmux send-keys -t $SESSION:3 "cd $ROOT/web && npm run dev" Enter

# 4: Go server
tmux new-window -t $SESSION:4 -n "server"
tmux send-keys -t $SESSION:4 "cd $ROOT/api && set -a && source .env && set +a && air" Enter

tmux select-window -t $SESSION:1
tmux attach -t $SESSION
