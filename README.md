# redis-pubsub-websocket

Simple proof of concept that bridges redis pubsub topics to websockets.

## Running

Start a redis server:
```
docker run --rm -p 6379:6379 --name redis redis
```

Start a couple of bridges. This will simulate a loadbalanced bridge:
``` 
./redis-pubsub-websocket --disable-origin-check --listen :8080 &
./redis-pubsub-websocket --disable-origin-check --listen :8081 &
```

Start a redis cli:
```
docker exec -it redis redis-cli
```

Open a chrome browser and paste this in to the inspector console. Note that ws1 subscribes to topic 1 & 2 while ws2 subscribes to topics 1 & 3.
```
var ws1 = new WebSocket('ws://localhost:8080/ws?t=mytopic.1&t=mytopic.2');
ws1.onmessage = function(e) { console.log("WS1 RECV: ", e.data) };
var ws2 = new WebSocket('ws://localhost:8081/ws?t=mytopic.1&t=mytopic.3');
ws2.onmessage = function(e) { console.log("WS2 RECV: ", e.data) };
```

Now in the previously opened redis cli try a few publishes:
```
PUBLISH mytopic.1 "topic 1 test"
PUBLISH mytopic.3 "topic 3 test"
PUBLISH mytopic.2 "topic 2 test"
```

In the chrome console you should see:
```
WS2 RECV:  topic 1 test
WS1 RECV:  topic 1 test
WS2 RECV:  topic 3 test
WS1 RECV:  topic 2 test
```
