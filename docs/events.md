Gate Control Agent Events
=========================

The GateControl Agent participates in the COLA message- and event eco-system. It
can react to requests, and also publishes information updates.

Broker setup
------------

At startup the following exchanges, queues and active bindings are established:

* __Topic exchanges__

  * `gatecontrol.event` - an exchange where gatecontrol events are published
  
* __Queues__

  * `gatecontrol-agent.[instance].command` a non-durable and non-persistent queue

* __Bindings__

  * `gatecontrol-agent.[instance].command => gatecontrol.event { 'gates.open' }`

    Establishes binding that ensures listening for open gate requests on the
    global `gatecontrol.event` exchange. Using the queue
    `gatecontrol-agent.[instance].command` for messages with the
    routing-key `gates.open`.

Events sent
-----------

The following events are actively emitted by the GateControl-Agent:

* `gatecontrol.agent.status` an agents status update

### Event `gatecontrol.agent.status`

Published on the `gatecontrol.event` exchange, containing updated agent status
information.

#### Message header

_No special header properties._

#### Message body

A JSON message payload, encoded as an UTF-8 string byte array.

```json
{
  "application": {
    "name": "gatecontrol-agent",
    "instance": 0
  },
  "terminal": {
    "locationCode": "DEKOB",
    "loadingPlaceId": 10000000001
  },
  "gates": [{"name": "entry-1","status": "UP"}],
  "scanners": [{
      "name": "scanner-1",
      "status": "UP"
    }, {
      "name": "scanner-2",
      "status": "DOWN"
  }]
}
```
