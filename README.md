# fluentbit-replica
fluentbit changes for replicaset 

Update the td-agent-bit.conf

Update container.conf file in this path: /etc/opt/microsoft/omsagent/<WSGUID>/conf/omsagent.d
restart omsagent: /opt/microsoft/omsagent/bin/service_control restart

Start fluent-bit with this conf file: /opt/td-agent-bit/bin/td-agent-bit -c /etc/td-agent-bit/td-agent-bit.conf


To build an output plugin:
You will need an so file generated.
Install and configure go using this link: 
https://www.linode.com/docs/development/go/install-go-on-ubuntu/ ( make sure to install version 1.10 and above or it will fail with string errors)
(or https://medium.com/@patdhlk/how-to-install-go-1-9-1-on-ubuntu-16-04-ee64c073cd79)
follow the first link to configure paths properly. 
Create folder in $HOME/go/src/KPI
Create a file out_kubepodinventory.go
Try go build.
If it fails with package not found errors: go get "pkg-name from errors"
go build -buildmode=c-shared -o out_kubepodinventory.so .
Try ldd out_kubepodinventory.so
Output should look like this: rashmi@rashmi-linux-go:~/go/src/KPI$ ldd out_kubepodinventory.so
        linux-vdso.so.1 =>  (0x00007fff527a3000)
        libpthread.so.0 => /lib/x86_64-linux-gnu/libpthread.so.0 (0x00007f9992e99000)
        libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f9992acf000)
        /lib64/ld-linux-x86-64.so.2 (0x00007f9994ff3000)
        
        
#install fluentbit on omsagent's pod

wget -qO - https://packages.fluentbit.io/fluentbit.key | sudo apt-key add -

sudo echo "deb https://packages.fluentbit.io/ubuntu/xenial xenial main" >> /etc/apt/sources.list  

sudo apt-get update

sudo apt-get install td-agent-bit sqlite3 libsqlite3-dev



#start fluent-bit

/opt/td-agent-bit/bin/td-agent-bit -c /etc/td-agent-bit/td-agent-bit.conf -e <path to.so file>
        

