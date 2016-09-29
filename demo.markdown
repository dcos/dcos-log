# DC/OS Logs API Demo
## Open Log Stream 
curl http://localhost:8080/stream           

## Get logs in different format
curl -H 'Accept: text/event-stream' http://localhost:8080/stream
curl -H 'Accept: application/json' http://localhost:8080/stream

## Print all logs    
curl http://52.36.170.119:8080/logs

## Limit 10 first messages from the beginning
curl -H 'Range: entries=:0:10' http://localhost:8080/logs

## Limit 10 last messages from the end
curl -H 'Range: entries=:-11:10' http://localhost:8080/logs

## Use a Query String for Filers
http://localhost:8080/logs?_PID=1




