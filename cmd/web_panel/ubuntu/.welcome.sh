#!/bin/bash
# 🌈 Fancy Welcome Script

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Fancy colorful ASCII banner
echo -e "
\033[38;5;196m ██░ ██ \033[38;5;160m▓█████  \033[38;5;124m██▓     \033[38;5;88m██▓     \033[38;5;52m▒█████   \033[38;5;16m▐██▌
\033[38;5;196m▓██░ ██▒\033[38;5;160m▓█   ▀ \033[38;5;124m▓██▒    \033[38;5;88m▓██▒    \033[38;5;52m▒██▒  ██▒ \033[38;5;16m▐██▌
\033[38;5;196m▒██▀▀██░\033[38;5;160m▒███   \033[38;5;124m▒██░    \033[38;5;88m▒██░    \033[38;5;52m▒██░  ██▒ \033[38;5;16m▐██▌
\033[38;5;196m░▓█ ░██ \033[38;5;160m▒▓█  ▄ \033[38;5;124m▒██░    \033[38;5;88m▒██░    \033[38;5;52m▒██   ██░ \033[38;5;16m▓██▒
\033[38;5;196m░▓█▒░██▓\033[38;5;160m░▒████▒\033[38;5;124m░██████▒\033[38;5;88m░██████▒\033[38;5;52m░ ████▓▒░ \033[38;5;16m▒▄▄ 
\033[38;5;196m ▒ ░░▒░▒\033[38;5;160m░░ ▒░ ░\033[38;5;124m░ ▒░▓  ░\033[38;5;88m░ ▒░▓  ░\033[38;5;52m░ ▒░▒░▒░  \033[38;5;16m░▀▀▒
\033[38;5;196m ▒ ░▒░ ░\033[38;5;160m ░ ░  ░\033[38;5;124m░ ░ ▒  ░\033[38;5;88m░ ░ ▒  ░\033[38;5;52m  ░ ▒ ▒░  \033[38;5;16m░  ░
\033[38;5;196m ░  ░░ ░\033[38;5;160m   ░   \033[38;5;124m ░ ░   \033[38;5;88m ░ ░   \033[38;5;52m░ ░ ░ ▒      ░
\033[38;5;196m ░  ░  ░\033[38;5;160m   ░  ░\033[38;5;124m   ░  ░\033[38;5;88m   ░  ░\033[38;5;52m    ░ ░   \033[38;5;16m░   
${NC}"
echo -e "${CYAN}---------------------------------------------${NC}"
echo -e "${CYAN} Welcome to server terminal! ${NC}"
echo -e "${GREEN} Logged in as:${NC} $USER"
echo -e "${CYAN} Enjoy your session and stay productive! ${NC}"

# Animated spinner while "loading"
spinner() {
    local delay=0.08
    local spinstr='|/-\'
    echo -n "Loading "
    for i in {1..20}; do
        local temp=${spinstr#?}
        printf " [%c]  " "$spinstr"
        spinstr=$temp${spinstr%"$temp"}
        sleep $delay
        printf "\b\b\b\b\b\b"
    done
    printf "    \b\b\b\b"
    echo "✅"
}
spinner

echo

# OS Info
if command -v lsb_release &> /dev/null; then
    os_name=$(lsb_release -d | cut -f2)
else
    os_name=$(uname -o)
fi
echo -e "${GREEN}OS:${NC} $os_name"

# Date & Time
echo -e "${YELLOW}Date & Time:${NC} $(date)"

# Hostname
echo -e "${YELLOW}Hostname:${NC} $(hostname)"

# Memory info
total_mem=$(free -h | awk '/^Mem:/ {print $2}')
used_mem=$(free -h | awk '/^Mem:/ {print $3}')
echo -e "${CYAN}Memory:${NC} $used_mem used / $total_mem total"

# CPU load
cpu_load=$(uptime | awk -F'load average:' '{ print $2 }' | cut -d',' -f1 | xargs)
echo -e "${CYAN}CPU Load:${NC} $cpu_load (1 min average)"

# Uptime
uptime_text=$(uptime -p)
echo -e "${CYAN}Uptime:${NC} $uptime_text"

# Disk usage
disk_usage=$(df -h / | awk 'NR==2 {print $3 " used / " $2 " total (" $5 " used)"}')
echo -e "${CYAN}Disk Usage (/):${NC} $disk_usage"

# Local IP
local_ip=$(hostname -I | awk '{print $1}')
echo -e "${CYAN}Local IP:${NC} $local_ip"

# Public IP
if command -v curl &> /dev/null; then
    public_ip=$(curl -s https://ipinfo.io/ip)
    [ -n "$public_ip" ] && echo -e "${CYAN}Public IP:${NC} $public_ip"
fi

# Docker containers
if command -v docker &> /dev/null; then
    docker_count=$(docker ps -q | wc -l)
    echo -e "${CYAN}Running Docker containers:${NC} $docker_count"
fi

# Top 3 CPU processes
echo -e "${YELLOW}Top 3 CPU processes:${NC}"
ps -eo pid,comm,%cpu --sort=-%cpu | awk 'NR>1 && NR<=4 {printf "  PID: %s  CMD: %s  CPU: %s%%\n", $1, $2, $3}'

# Fancy progress bar (simulate loading)
echo -ne "${MAGENTA}Loading complete:${NC} "
for i in {1..20}; do
    echo -ne "▇"
    sleep 0.03
done
echo -e " ✅"

echo -e "${MAGENTA}-------------------------------------------${NC}"
