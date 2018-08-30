Dedupe bridge cni plugin
========================

This plugin is meant as a secondary plugin for bridge plugins in promisc mode.

It is a copy of some bells and whistles the kubernetes `kubenet` plugin is doing to to make the bridge plugin work correctly that are not available in then using cni networking in kubernetes.


How to use
==========
Just chain this plugin after a bridge plugin definition:

```

[
{
  "type": "bridge",
  "bridge": "cbr0",
  "mtu": 9000,
  "addIf": "eth0",
  "isGateway": true,
  "ipMasq": false,
  "hairpinMode": false,
  "promiscMode": true,
  "ipam": {
    "type": "host-local",
    "subnet": "100.80.0.0/23",
    "gateway": "100.80.0.1",
    "routes": [
      { "dst": "0.0.0.0/0" }
    ]
  }
},
{
  cniVersion:"0.3.0",
  name: "promisc-bridge",
  type:"dedup-bridge",
  device:"cbr0",
}
]
```
