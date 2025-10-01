# Trust Assessment Framework


This repository provides the latest prototype of the standalone Trust Assessment Framework.


If you are using the TAF prototype for your own research, please use the following citation:

> Trkulja, N., Hermann, A., Duhr, P.L., Meißner, E., Buchholz, M., Kargl, F. and Erb, B. 2025. Vehicle-to-Everything Trust: Enabling Autonomous Trust Assessment of V2X Data by Vehicles. *Proceedings of the 2025 Cyber Security in CarS Workshop (Taipei, Taiwan, 2025)*.
```
@inproceedings{Trkulja2025trust,
	author = {Trkulja, Nata{\v{s}}a and Hermann, Artur and Duhr, Paul L. and Meißner, Echo and Buchholz, Michael and Kargl, Frank and Erb, Benjamin},
	title = {Vehicle-to-Everything Trust: Enabling Autonomous Trust Assessment of V2X Data by Vehicles},
	year = {2025},
	booktitle = {Proceedings of the 2025 Cyber Security in CarS Workshop},
	location = {Taipei, Taiwan},
	series = {CSCS '25},
}
```


## Gettting Started

### Build from Source

Clone this repository:
```shell
git clone git@github.com:vs-uulm/go-taf.git
```

Next, go to the `go-taf` directory and run make:

```shell
cd go-taf
make build
```

To run the TAF, you can also use make: 

```shell
make run
```

To build and run the TAF with an enabled debugging webinterface, you can use the following make command:

```shell
GOFLAGS=-tags=webui make run
```

## Configuration

The TAF uses an internal configuration with hardcoded defaults. To change the configuration, you can use a JSON file (template located in `res/taf.json`) and specify the actual file location in the environment variable `TAF_CONFIG`. The following options can be configured. Missing options are implicitly using their defined default values.

```js
{
  "Identifier": "taf",                  // internal identifier of this instance 
  "Communication": {
    "Kafka": {
      "Broker": "localhost:9092",       // address and port of the kafka bootstrap server
      "TafTopic": "taf"                 // kafka topic the TAF will consume
    },
    "TafEndpoint": "taf",               // kafka identifier of TAF component
    "AivEndpoint": "aiv",               // kafka identifier of AIV component
    "MbdEndpoint": "mbd"                // kafka identifier of MBD component
  },
  "Logging": {
    "LogLevel": 2,                      // log level: 1=TRACE, 2=DEBUG, 3=INFO,
                                        //    4=WARN, 5=ERROR, 6=FATAL, 7=PRINT
    "LogStyle": "PRETTY"                // log style: 'PRETTY', 'JSON', or 'PLAIN'
  },
  "Debug": {
    "FixedSessionID": "",               // if provided, this fixed value is used by the TAM
                                        // instead of a random UUID-based session id
    "FixedSubscriptionID": "",          // if provided, this fixed value is used by the TAM
                                        // instead of a random UUID-based subscription id
    "FixedRequestID": ""                // if provided, this fixed request id is used by the
                                        // trust source manager instead of a random UUID-based id
  },
  "V2X" : {
    "NodeTTLsec" : 5,                   // The time to live of a node (vehicle) in seconds based on CPMs.
                                        // If there is no message after that time span, the vehicle
                                        // is considered to be gone. 
    "CheckIntervalSec" : 1              // The interval in seconds how often vehicles should be checked
                                        // for TTL expiries (see above).
  }
}
```




## Updating Message Schema and Auto-Generating Go Structs

**Warning:** *This step is only necessary after modifying existing schemas or adding new schemas. **Don't do this step unless you know that it is really necessary, as it overwrites existing code and may break the existing TAF implementation.***

This step requires `quicktype`. Having node/npm already installed, you can install it using:

```shell
npm install -g quicktype
```

All JSON schemas are located in the folder `res/schemas/`.
By running the command below, corresponding Go structs will be generated into the directory `pkg/message/<namespace>/`. 

```shell
make generate-structs 
```

To remove existing structs, you can use the following command:

```shell
make clean-structs 
```

Again, please note that adding new schemas/structs will require manual code changes in addition to the auto-generation of the structs.

