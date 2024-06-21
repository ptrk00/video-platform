package authz

import input.token as token

default allow = false

allow {
    [isValid, header, payload] := io.jwt.decode_verify(token, {"secret": "supersecretkey"})
    payload.username == "user1"
}