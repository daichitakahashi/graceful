# graceful
graceful listener

# Usage
```
rawLn, _ := net.Listener("tcp", addr)
ln := graceful.Upgrade(rawLn)
```
