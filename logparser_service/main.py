# logparser_service/main.py
from fastapi import FastAPI, File, UploadFile, HTTPException, Form
from fastapi.responses import JSONResponse
from logparser.Drain import LogParser
import tempfile
import os
import pandas as pd
import shutil
import json
from datetime import datetime

app = FastAPI(title="Logparser Microservice", version="1.0.0")

@app.post("/parse")
async def parse_log(file: UploadFile = File(...), log_format: str = Form(None)):
    try:
        with tempfile.TemporaryDirectory() as tmpdir:
            file_path = os.path.join(tmpdir, file.filename)
            with open(file_path, "wb") as f:
                shutil.copyfileobj(file.file, f)
            # Read lines and try to parse as JSON
            parsed_entries = []
            debug_count = 0
            with open(file_path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    if debug_count < 10:
                        print(f"[DEBUG] Trying to parse line: {repr(line)}")
                    try:
                        obj = json.loads(line)
                        entry = {
                            "timestamp": obj.get("timestamp") or obj.get("time"),
                            "level": obj.get("level") or obj.get("log_level"),
                            "message": obj.get("message") or obj.get("msg"),
                            "metadata": obj.get("metadata") or obj.get("data"),
                            "rawData": line
                        }
                        if entry["timestamp"]:
                            try:
                                dt = datetime.fromisoformat(entry["timestamp"].replace("Z", "+00:00"))
                                entry["timestamp"] = dt.isoformat()
                            except Exception:
                                pass
                        parsed_entries.append(entry)
                    except Exception as e:
                        if debug_count < 10:
                            print(f"[DEBUG] JSON parse error: {e} for line: {repr(line)}")
                        continue
                    debug_count += 1
            if parsed_entries:
                print(f"[DEBUG] Parsed {len(parsed_entries)} JSON entries.")
                return JSONResponse(content=parsed_entries)
            # If no JSON lines found, fallback to logparser
            if not log_format:
                log_format = '<Date> <Time> <Level>:<Content>'
            parser = LogParser(log_format, indir=tmpdir, outdir=tmpdir, depth=4, st=0.5)
            parser.parse(file.filename)
            structured_path = os.path.join(tmpdir, file.filename + "_structured.csv")
            if not os.path.exists(structured_path):
                raise HTTPException(status_code=500, detail="Logparser failed to produce structured output.")
            df = pd.read_csv(structured_path)
            return JSONResponse(content=df.to_dict(orient="records"))
    except Exception as e:
        print(f"[DEBUG] Exception in /parse: {e}")
        raise HTTPException(status_code=500, detail=str(e)) 