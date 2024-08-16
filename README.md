realip
=======
![](https://github.com/lestrrat-go/realip/workflows/CI/badge.svg) [![Go Reference](https://pkg.go.dev/badge/github.com/lestrrat-go/realip.svg)](https://pkg.go.dev/github.com/lestrrat-go/realip) 


Forked from [github.com/natureglobal/realip](https://github.com/natureglobal/realip).

Differences:
* More idiomatic Go style
* Avoids closures
* Caches trusted IPs

I tried overhauling in hopes of gaining significant performance boost, but as of this writing the differences matters only ever so slightly over the orignal.