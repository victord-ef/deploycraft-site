---
title: "Linux"
description: "Linux commands for file operations, process management, networking, user administration, and text processing."
icon: "🐧"
weight: 12
count: 70
tags: ["linux", "bash", "sysadmin"]
---

## File & Directory

```bash
ls -la
ls -lhS                                   # sort by size
find / -name "*.conf" 2>/dev/null
find / -type f -mmin -10                  # modified in last 10 min
find / -type f -perm /4000 2>/dev/null    # SUID files
find / -writable -type d 2>/dev/null      # world-writable dirs
locate <filename>
which <command>
whereis <command>
file <file>
stat <file>
du -sh *                                  # directory sizes
du -sh * | sort -rh | head -10
df -h
df -ih                                    # inode usage
```

## File Operations

```bash
cp <src> <dst>
cp -r <src_dir> <dst_dir>
mv <src> <dst>
rm <file>
rm -rf <dir>
mkdir -p /path/to/dir
ln -s <target> <link>                     # symlink
ln <target> <link>                        # hard link
chmod 755 <file>
chmod +x <file>
chmod -R 644 /path/
chown user:group <file>
chown -R user:group /path/
umask
touch <file>
truncate -s 0 <file>                      # empty a file
```

## File Content

```bash
cat <file>
less <file>
head -n 20 <file>
tail -n 20 <file>
tail -f <file>                            # follow
tail -f <file> | grep "ERROR"
wc -l <file>
sort <file>
sort -u <file>                            # unique
sort -rn <file>                           # reverse numeric
uniq -c <file>                            # count duplicates
cut -d: -f1 /etc/passwd
awk '{print $1}' <file>
awk -F: '{print $1,$6}' /etc/passwd
sed 's/old/new/g' <file>
sed -i 's/old/new/g' <file>              # in-place
grep "pattern" <file>
grep -r "pattern" /path/
grep -i "pattern" <file>                 # case insensitive
grep -v "pattern" <file>                 # invert
grep -n "pattern" <file>                 # line numbers
grep -l "pattern" /path/*               # files with match
xargs
```

## Archives

```bash
tar -czf archive.tar.gz /path/
tar -xzf archive.tar.gz
tar -xzf archive.tar.gz -C /target/
tar -tzf archive.tar.gz                  # list contents
zip -r archive.zip /path/
unzip archive.zip
unzip archive.zip -d /target/
gzip <file>
gunzip <file>.gz
```

## Process Management

```bash
ps aux
ps aux | grep <name>
pgrep <name>
pkill <name>
kill <pid>
kill -9 <pid>                            # force kill
killall <name>
top
htop
jobs
bg %1
fg %1
nohup <command> &
screen -S <name>
screen -ls
screen -r <name>
tmux new -s <name>
tmux ls
tmux attach -t <name>
```

## systemd / Services

```bash
systemctl status <service>
systemctl start <service>
systemctl stop <service>
systemctl restart <service>
systemctl reload <service>
systemctl enable <service>
systemctl disable <service>
systemctl is-active <service>
systemctl list-units --type=service
systemctl list-units --failed
journalctl -u <service>
journalctl -u <service> -f              # follow
journalctl -u <service> --since "1 hour ago"
journalctl -p err                        # errors only
```

## Networking

```bash
ip addr
ip addr show eth0
ip route
ip route add 192.168.2.0/24 via 192.168.1.1
ip link set eth0 up
ss -tuln
ss -tunp
netstat -tuln
curl -I <url>
curl -X POST -H "Content-Type: application/json" -d '{}' <url>
wget <url>
wget -O <file> <url>
ping -c 4 <host>
traceroute <host>
nslookup <domain>
dig <domain>
```

## User & Group Management

```bash
useradd <user>
useradd -m -s /bin/bash <user>
usermod -aG sudo <user>
userdel -r <user>
passwd <user>
id <user>
whoami
w
last
lastlog
cat /etc/passwd
cat /etc/shadow
getent passwd <user>
groupadd <group>
groupdel <group>
groups <user>
```

## Sudo & Permissions

```bash
sudo -l                                  # list sudo privileges
sudo -u <user> <command>
sudo su -
visudo
cat /etc/sudoers
find / -perm -4000 2>/dev/null          # SUID
find / -perm -2000 2>/dev/null          # SGID
getfacl <file>
setfacl -m u:<user>:rwx <file>
```

## Environment & Shell

```bash
env
printenv
export VAR=value
echo $VAR
history
history | grep <keyword>
!<number>                                # run history command by number
source ~/.bashrc
alias ll='ls -la'
cron - crontab -l
crontab -e
```
