# go-cookie-session

Implementation example for understanding the usage of HTTPOnly cookies in authentication, using redis/kvrocks for storing session information.

Implemented using the standard library and fiber, which can be changed in the `main.go` file for testing.

```go
err := StartStdMuxServer(cfg)
// err := StartFiberServer(cfg)
if err != nil {
    logger.Fatal("failed to start server", err)
}
```

**Versions**
- **Go:** 1.22.4
- **kvrocks** 2.9.0

## Running

Start the `kvrocks/redis` container using `compose.yaml`
```bash
docker-compose up -d
```

Starts the server at [http://localhost:8888/](http://localhost:8888/)
```
go run ./server
```

## Routes

`public/index.html` has a form and buttons to test each request and cookie changes.

| Route | function |
| ----- | -------- |
 **/** | testing page |
|**/login**|validate credentials and stores session information |
|**/refresh**| refreshes the session using the refresh cookie |
| **/protected** | protected route requiring authentication |
| **/clear-cookies** | clears all cookies |
| **/clear-session** | clears session cookie |
| **/clear-refresh** | clears refresh cookie |