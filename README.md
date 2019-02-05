# observe
Observe wrapper around opencensus and logrus for holy trinity integration -- metrics, logs and traces

Status: **PreAlpha**

## Design goals

* Simple
* low overhead 
* Easy instrumeting function which last `>1ms`
* Thin wrapper around specified libraries

Currently there are two possibilities depending on your standard:

## OpenTracing

located in `/ot` folders: It's a opinionated wrapper around OpenTracing, prometheus and logrus. 
Currently more work is being put into that implementation.

## OpenCensus

located in `/oc` folders: It's a opinionated wrapper around OpenCensus, prometheus and logrus. 


## Examples

See examples folder for usage. I recommend also reading the source if you're using it. 
