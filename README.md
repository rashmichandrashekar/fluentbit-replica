# fluentbit-replica
fluentbit changes for replicaset 

Update the td-agent-bit.conf

Update container.conf file in this path: /etc/opt/microsoft/omsagent/<WSGUID>/conf/omsagent.d
restart omsagent: /opt/microsoft/omsagent/bin/service_control restart

Start fluent-bit with this conf file: /opt/td-agent-bit/bin/td-agent-bit -c /etc/td-agent-bit/td-agent-bit.conf
