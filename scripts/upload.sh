
curl -vv -X POST http://localhost:8080/upload -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6InVzZXIxIiwiaWQiOjEsImV4cCI6MTcxOTUwODU0Mn0.OIbdxK6P30XkKlyfotvRWdplRAqJF7nUi_eKcI8V_g4' -F "myfile=@login.sh"
# curl -X POST http://localhost:8080/upload -H 'Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6InVzZXIyIiwiaWQiOjIsImV4cCI6MTcxOTA3Nzg3M30.pUnV5smoTC-GiSnmCrhM03_vDrCtLH_Ikau50BZWQHk' -F "myfile=@login.sh"
