import socket,ssl,time
ctx=ssl.create_default_context()
for host in ['qzweb.cloud','www.qzweb.cloud']:
    for i in range(3):
        t=time.monotonic()
        try:
            with socket.create_connection(('124.220.2.69',443),5) as raw:
                with ctx.wrap_socket(raw,server_hostname=host) as s:
                    print(host,i+1,'TLS_OK',s.version(),s.getpeercert().get('notAfter'))
                    s.sendall(('GET / HTTP/1.1\r\nHost: '+host+'\r\nConnection: close\r\n\r\n').encode())
                    print(s.recv(128).split(b'\r\n')[0].decode())
        except Exception as e:
            print(host,i+1,type(e).__name__,str(e))
        print('seconds',round(time.monotonic()-t,3))
