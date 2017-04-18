# Market Streamer
[![Build Status](https://drone.element-43.com/api/badges/EVE-Tools/market-streamer/status.svg)](https://drone.element-43.com/EVE-Tools/market-streamer) [![Go Report Card](https://goreportcard.com/badge/github.com/eve-tools/market-streamer)](https://goreportcard.com/report/github.com/eve-tools/market-streamer)

This service for [Element43](https://element-43.com) provides a drop-in replacement for [EMDR](http://www.eve-emdr.com/en/latest/). It fetches market data from [ESI](https://esi.tech.ccp.is/latest/) and provides a ZMQ socket compatible with EMDR's output format based on [UUDIF](http://dev.eve-central.com/unifieduploader/start). On the first run updates are spread over five minutes. Subsequent requests are made when the region's cache in ESI expires (every five minutes). The region's data is augmented with data for publicly accessible (depending on the token you supply) structures (citadels). Each message on the ZeroMQ socket contains a whole region. Types with no orders yield an empty list of rows inside the result set (see UUDIF docs). De-duplication by downstream consumers can be achieved by hashing the individual rowset's rows and comparing hashes with past values. See [emdr-to-nsq](https://github.com/EVE-Tools/emdr-to-nsq) for an example.

## Obtaining a refresh Token

* Create an application on https://developers.eveonline.com - for scopes choose
  * `esi-universe.read_structures.v1`
  * `esi-markets.structure_markets.v1`
* You can use any SSO script for obtaining a token
* For fetching the token with (evedata)[https://github.com/antihax/evedata] use something like http://localhost:3000/X/boostrapEveSSOAnswer as callback URL
* Note the client ID and secret and configure evedata with it
* Use the route http://localhost:3000/X/boostrapEveAuth for obtaining your token
* Configure the market streamer with your ID, secret and refresh token
* Set your callback URL to something like `eveauth-e43://market-streame` - you won't need it anymore

## Installation
Either use the prebuilt Docker images and pass the appropriate env vars (see below), or:

* Clone this repo into your gopath
* Run `go get`
* Run `go build`

## Deployment Info
Builds and releases are handled by Drone.

Environment Variable | Default | Description
--- | --- | ---
LOG_LEVEL | info | Threshold for logging messages to be printed
CLIENT_ID | `none` | Required - your 3rd party app's client ID - get it from https://developers.eveonline.com
SECRET_KEY | `none` | Required - your 3rd party app's secret key - get it from https://developers.eveonline.com
REFRESH_TOKEN | `none` | Required - A valid refresh token - see above docs for generating one
ZMQ_BIND_ENDPOINT | tcp://127.0.0.1:8050 | The ZMQ enpoint will bind to this address you could use `tcp://*:8050`to listen on any address
LOCATION_SERVICE_URL | https://element-43.com/api/static-data/v1/location/ | URL of service providing location info - see [static-data](https://github.com/EVE-Tools/static-data)