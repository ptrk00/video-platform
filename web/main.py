from fastapi import FastAPI, Request, Form, UploadFile, File, HTTPException, Depends, Query
from fastapi.responses import HTMLResponse, RedirectResponse, StreamingResponse
from fastapi.templating import Jinja2Templates
import httpx
import io
import jwt

JWT_SECRET = "supersecretkey"

app = FastAPI()
templates = Jinja2Templates(directory="templates")

uploader_service_url = "http://uploader:8080"

async def get_user_id_from_token(token: str):
    token = token.split(" ")[1]
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=["HS256"])
        return payload.get("id"), payload.get("username")
    except (jwt.ExpiredSignatureError, jwt.InvalidTokenError) as e:
        print(e)
        return None, None

@app.get("/", response_class=HTMLResponse)
async def home(request: Request):
    token = request.cookies.get("Authorization")
    user_id, username = await get_user_id_from_token(token) if token else (None, None)
    return templates.TemplateResponse("home.html", {"request": request, "username": username})

@app.get("/login", response_class=HTMLResponse)
async def login_get(request: Request):
    token = request.cookies.get("Authorization")
    user_id, username = await get_user_id_from_token(token) if token else (None, None)
    return templates.TemplateResponse("login.html", {"request": request, "username": username})

@app.post("/login", response_class=HTMLResponse)
async def login_post(request: Request, username: str = Form(...), password: str = Form(...)):
    async with httpx.AsyncClient() as client:
        response = await client.post(f"{uploader_service_url}/login", json={"username": username, "password": password})
    if response.status_code == 200:
        token = response.json().get("token")
        response = RedirectResponse(url="/upload", status_code=302)
        response.set_cookie(key="Authorization", value=f"Bearer {token}")
        return response
    return templates.TemplateResponse("login.html", {"request": request, "error": "Invalid credentials"})

@app.get("/upload", response_class=HTMLResponse)
async def upload_get(request: Request):
    token = request.cookies.get("Authorization")
    user_id, username = await get_user_id_from_token(token) if token else (None, None)
    return templates.TemplateResponse("upload.html", {"request": request, "username": username})

@app.post("/upload", response_class=HTMLResponse)
async def upload_post(request: Request, file: UploadFile = File(...)):
    token = request.cookies.get("Authorization")
    if not token:
        return RedirectResponse(url="/login", status_code=302)

    files = {"myfile": (file.filename, file.file, file.content_type)}
    headers = {"Authorization": token}
    async with httpx.AsyncClient() as client:
        response = await client.post(f"{uploader_service_url}/upload", files=files, headers=headers)
    if response.status_code == 200:
        return templates.TemplateResponse("upload.html", {"request": request, "message": "File uploaded successfully"})
    return templates.TemplateResponse("upload.html", {"request": request, "error": "Failed to upload file"})

@app.get("/files", response_class=HTMLResponse)
async def files_get(request: Request):
    token = request.cookies.get("Authorization")
    if not token:
        return RedirectResponse(url="/login", status_code=302)
    
    user_id, username = await get_user_id_from_token(token) if token else (None, None)

    headers = {"Authorization": token}
    async with httpx.AsyncClient() as client:
        response = await client.get(f"{uploader_service_url}/files", headers=headers)
    if response.status_code == 200:
        files = response.json()
        return templates.TemplateResponse("files.html", {"request": request, "files": files, "username": username})
    return templates.TemplateResponse("files.html", {"request": request, "error": "Failed to fetch files"})

@app.get("/download", response_class=HTMLResponse)
async def download_post(request: Request, etag: str = Query(...), archived: bool = Query(...)):
    token = request.cookies.get("Authorization")
    if not token:
        return RedirectResponse(url="/login", status_code=302)

    headers = {"Authorization": token}
    async with httpx.AsyncClient() as client:
        response = await client.get(f"{uploader_service_url}/download", params={"etag": etag, "archived": archived}, headers=headers)
    
    if response.status_code == 200:
        file_content = response.content
        filename = response.headers.get("Content-Disposition").split("filename=")[-1]
        return StreamingResponse(io.BytesIO(file_content), media_type="application/octet-stream", headers={
            "Content-Disposition": f"attachment; filename={filename}"
        })
    return templates.TemplateResponse("download.html", {"request": request, "error": "Failed to download file"})