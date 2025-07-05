# logparser_service/main.py
from fastapi import FastAPI, File, UploadFile, HTTPException, Form
from fastapi.responses import JSONResponse
from logparser.Drain import LogParser
import tempfile
import os
import pandas as pd
import shutil
import json
import re
from datetime import datetime
from enhanced_ml_parser import ml_parser

app = FastAPI(title="Logparser Microservice", version="1.0.0")

def normalize_timestamp(timestamp_str):
    """Normalize timestamp to ISO format"""
    if not timestamp_str:
        return None
    
    # Common timestamp formats
    formats = [
        "%Y-%m-%dT%H:%M:%S.%fZ",
        "%Y-%m-%dT%H:%M:%SZ", 
        "%Y-%m-%dT%H:%M:%S.%f",
        "%Y-%m-%dT%H:%M:%S",
        "%Y-%m-%d %H:%M:%S.%f",
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%d %H:%M:%S.%f%z",
        "%Y-%m-%d %H:%M:%S%z",
    ]
    
    # Remove timezone info for parsing, then add back
    ts_clean = timestamp_str.replace("Z", "").replace("+00:00", "")
    
    for fmt in formats:
        try:
            dt = datetime.strptime(ts_clean, fmt)
            return dt.isoformat()
        except ValueError:
            continue
    
    # Try ISO format directly
    try:
        dt = datetime.fromisoformat(timestamp_str.replace("Z", "+00:00"))
        return dt.isoformat()
    except Exception:
        pass
    
    return timestamp_str

def extract_json_fields(obj):
    """Extract and normalize fields from JSON object with synonym support"""
    # Field synonyms for robust extraction
    timestamp_synonyms = ["timestamp", "ts", "time", "date", "datetime", "@timestamp"]
    level_synonyms = ["level", "severity", "log_level", "lvl", "priority"]
    message_synonyms = ["message", "msg", "log", "log_message", "text", "body", "content"]
    metadata_synonyms = ["metadata", "meta", "extra", "details", "data", "context"]
    
    entry = {
        "timestamp": None,
        "level": None, 
        "message": None,
        "metadata": {},
        "rawData": json.dumps(obj)
    }
    
    # Extract timestamp
    for key in timestamp_synonyms:
        if key in obj and obj[key]:
            entry["timestamp"] = normalize_timestamp(str(obj[key]))
            break
    
    # Extract level
    for key in level_synonyms:
        if key in obj and obj[key]:
            level = str(obj[key]).upper().strip()
            # Normalize level
            if level in ["WARN", "WARNING"]:
                level = "WARN"
            elif level in ["ERR", "ERROR"]:
                level = "ERROR"
            elif level in ["CRIT", "CRITICAL", "FATAL"]:
                level = "FATAL"
            elif level in ["DBG", "DEBUG"]:
                level = "DEBUG"
            elif level in ["INF", "INFO"]:
                level = "INFO"
            entry["level"] = level
            break
    
    # Extract message
    for key in message_synonyms:
        if key in obj and obj[key]:
            entry["message"] = str(obj[key])
            break
    
    # Extract metadata (all other fields)
    for key, value in obj.items():
        if key not in timestamp_synonyms + level_synonyms + message_synonyms:
            if isinstance(value, (dict, list)):
                entry["metadata"][key] = value
            else:
                entry["metadata"][key] = str(value)
    
    return entry

def parse_hybrid_json(lines):
    """Parse lines that may contain both JSON and unstructured content"""
    parsed_entries = []
    json_count = 0
    unstructured_lines = []
    
    for idx, line in enumerate(lines):
        line = line.strip()
        if not line:
            continue
            
        try:
            obj = json.loads(line)
            entry = extract_json_fields(obj)
            parsed_entries.append(entry)
            json_count += 1
        except json.JSONDecodeError:
            # Not JSON, collect for ML parsing
            unstructured_lines.append(line)
    
    print(f"[DEBUG] Hybrid parsing: {json_count} JSON lines, {len(unstructured_lines)} unstructured lines")
    return parsed_entries, unstructured_lines

def extract_structured_fields_from_ml(content, timestamp=None, level=None):
    """Extract structured fields from ML-parsed content using advanced regex patterns"""
    entry = {
        "timestamp": timestamp,
        "level": level,
        "message": content,
        "metadata": {},
        "rawData": content
    }
    
    # Enhanced regex patterns for various log types
    patterns = [
        # Network and IP patterns
        (r'\b(?:\d{1,3}\.){3}\d{1,3}\b', 'ip_address'),
        (r'\b(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}\b', 'ipv6_address'),
        
        # HTTP patterns
        (r'\b(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^\s]+)', 'http_path'),
        (r'\b(\d{3})\s+(?:OK|ERROR|NOT_FOUND|FORBIDDEN|UNAUTHORIZED|BAD_REQUEST|INTERNAL_SERVER_ERROR)', 'http_status'),
        (r'HTTP/(\d+\.\d+)', 'http_version'),
        (r'User-Agent:\s*([^\r\n]+)', 'user_agent'),
        (r'Referer:\s*([^\r\n]+)', 'referer'),
        (r'Content-Length:\s*(\d+)', 'content_length'),
        
        # Request/Response patterns
        (r'request_id[=:]\s*([a-zA-Z0-9-_]+)', 'request_id'),
        (r'correlation_id[=:]\s*([a-zA-Z0-9-_]+)', 'correlation_id'),
        (r'trace_id[=:]\s*([a-zA-Z0-9-_]+)', 'trace_id'),
        (r'session_id[=:]\s*([a-zA-Z0-9-_]+)', 'session_id'),
        
        # Database patterns
        (r'SQL:\s*([^;]+)', 'sql_query'),
        (r'database[=:]\s*([a-zA-Z0-9_-]+)', 'database_name'),
        (r'table[=:]\s*([a-zA-Z0-9_-]+)', 'table_name'),
        (r'query_time[=:]\s*([0-9.]+)', 'query_time'),
        
        # Performance patterns
        (r'execution_time[=:]\s*([0-9.]+)', 'execution_time'),
        (r'response_time[=:]\s*([0-9.]+)', 'response_time'),
        (r'memory_usage[=:]\s*([0-9.]+)', 'memory_usage'),
        (r'cpu_usage[=:]\s*([0-9.]+)', 'cpu_usage'),
        
        # Error patterns
        (r'error_code[=:]\s*([a-zA-Z0-9_-]+)', 'error_code'),
        (r'exception[=:]\s*([a-zA-Z0-9_.]+)', 'exception_type'),
        (r'stack_trace[=:]\s*([^\r\n]+)', 'stack_trace'),
        
        # Application patterns
        (r'user[=:]\s*([a-zA-Z0-9_-]+)', 'user'),
        (r'tenant[=:]\s*([a-zA-Z0-9_-]+)', 'tenant'),
        (r'environment[=:]\s*([a-zA-Z0-9_-]+)', 'environment'),
        (r'version[=:]\s*([0-9.]+)', 'version'),
        
        # Docker/Kubernetes patterns
        (r'container[=:]\s*([a-zA-Z0-9_-]+)', 'container'),
        (r'pod[=:]\s*([a-zA-Z0-9_-]+)', 'pod'),
        (r'namespace[=:]\s*([a-zA-Z0-9_-]+)', 'namespace'),
        (r'node[=:]\s*([a-zA-Z0-9_-]+)', 'node'),
        
        # File/Path patterns
        (r'file[=:]\s*([^\s]+)', 'file_path'),
        (r'line[=:]\s*(\d+)', 'line_number'),
        (r'function[=:]\s*([a-zA-Z0-9_]+)', 'function_name'),
        (r'module[=:]\s*([a-zA-Z0-9_.]+)', 'module_name'),
        
        # Process/Thread patterns
        (r'process_id[=:]\s*(\d+)', 'process_id'),
        (r'thread_id[=:]\s*(\d+)', 'thread_id'),
        (r'thread_name[=:]\s*([a-zA-Z0-9_-]+)', 'thread_name'),
        
        # Time patterns
        (r'duration[=:]\s*([0-9.]+)', 'duration'),
        (r'elapsed[=:]\s*([0-9.]+)', 'elapsed_time'),
        
        # Size/Count patterns
        (r'size[=:]\s*(\d+)', 'size'),
        (r'count[=:]\s*(\d+)', 'count'),
        (r'bytes[=:]\s*(\d+)', 'bytes'),
        
        # URL patterns
        (r'https?://[^\s]+', 'url'),
        (r'ws://[^\s]+', 'websocket_url'),
        
        # Email patterns
        (r'\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b', 'email'),
        
        # UUID patterns
        (r'\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b', 'uuid'),
        
        # Hash patterns
        (r'\b[a-f0-9]{32}\b', 'md5_hash'),
        (r'\b[a-f0-9]{40}\b', 'sha1_hash'),
        (r'\b[a-f0-9]{64}\b', 'sha256_hash'),
    ]
    
    # Extract fields using patterns
    for pattern, field_name in patterns:
        matches = re.findall(pattern, content, re.IGNORECASE)
        if matches:
            if len(matches) == 1:
                entry["metadata"][field_name] = matches[0]
            else:
                entry["metadata"][field_name] = matches
    
    # Try to extract level from content if not provided
    if not entry["level"]:
        level_patterns = [
            r'\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)\b',
            r'\[(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)\]',
            r'level[=:]\s*(DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL|TRACE)',
            r'\[(INFO|WARN|ERROR|FATAL|CRITICAL)\]',
            r'(INFO|WARN|ERROR|FATAL|CRITICAL)[:\s]',
        ]
        for pattern in level_patterns:
            match = re.search(pattern, content, re.IGNORECASE)
            if match:
                level = match.group(1).upper()
                if level in ["WARN", "WARNING"]:
                    level = "WARN"
                elif level in ["ERR", "ERROR"]:
                    level = "ERROR"
                elif level in ["CRIT", "CRITICAL", "FATAL"]:
                    level = "FATAL"
                elif level in ["DBG", "DEBUG"]:
                    level = "DEBUG"
                elif level in ["INF", "INFO"]:
                    level = "INFO"
                entry["level"] = level
                break
    
    # Try to extract timestamp from content if not provided
    if not entry["timestamp"]:
        timestamp_patterns = [
            r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})',
            r'\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?',
            r'\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]',
            r'(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})',
            r'\[(\d{2}/\w{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4})\]',
            r'(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})',
        ]
        for pattern in timestamp_patterns:
            match = re.search(pattern, content)
            if match:
                ts_str = match.group(1) if len(match.groups()) > 0 else match.group(0)
                entry["timestamp"] = normalize_timestamp(ts_str)
                break
    
    return entry

def detect_log_format(lines):
    """Detect log format from sample lines"""
    if not lines:
        return '<Date> <Time> <Level>: <Content>'
    
    # Sample first few lines for format detection
    sample_lines = lines[:min(5, len(lines))]
    
    # Common log format patterns
    format_patterns = [
        # RFC3339 with level
        (r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})\s+(?:DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL)', 
         '<Date> <Time> <Level> <Content>'),
        
        # Standard syslog format
        (r'^[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+\S+\s+\S+:', 
         '<Date> <Time> <Host> <Process>: <Content>'),
        
        # Java log format
        (r'^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2},\d{3}\s+(?:DEBUG|INFO|WARN|WARNING|ERROR|FATAL)', 
         '<Date> <Time> <Level> <Content>'),
        
        # Python log format
        (r'^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2},\d{3}\s+-\s+(?:DEBUG|INFO|WARN|WARNING|ERROR|CRITICAL)', 
         '<Date> <Time> - <Level> <Content>'),
        
        # Apache/Nginx access log - handle specially
        (r'^\S+\s+\S+\s+\S+\s+\[[^\]]+\]\s+"[^"]+"\s+\d{3}\s+\d+', 
         'apache_nginx'),
        
        # Docker log format
        (r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})\s+\S+', 
         '<Date> <Container> <Content>'),
        
        # Kubernetes log format
        (r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})\s+\S+\s+\S+', 
         '<Date> <Pod> <Container> <Content>'),
    ]
    
    # Count matches for each format
    format_scores = {}
    for pattern, format_str in format_patterns:
        matches = 0
        for line in sample_lines:
            if re.match(pattern, line):
                matches += 1
        if matches > 0:
            format_scores[format_str] = matches / len(sample_lines)
    
    # Return the format with highest score, or default
    if format_scores:
        best_format = max(format_scores.items(), key=lambda x: x[1])
        if best_format[1] >= 0.6:  # At least 60% match
            print(f"[DEBUG] Detected log format: {best_format[0]} (confidence: {best_format[1]:.2f})")
            return best_format[0]
    
    print("[DEBUG] Using default log format")
    return '<Date> <Time> <Level>: <Content>'

def parse_apache_nginx_logs(lines):
    """Special parser for Apache/Nginx access logs"""
    entries = []
    
    for line in lines:
        # Apache/Nginx access log pattern
        pattern = r'^(\S+)\s+(\S+)\s+(\S+)\s+\[([^\]]+)\]\s+"([^"]+)"\s+(\d{3})\s+(\d+)(?:\s+"([^"]*)"\s+"([^"]*)")?'
        match = re.match(pattern, line)
        
        if match:
            ip, user1, user2, timestamp, request, status, size, referer, user_agent = match.groups()
            
            # Parse HTTP request
            request_parts = request.split()
            method = request_parts[0] if len(request_parts) > 0 else ""
            path = request_parts[1] if len(request_parts) > 1 else ""
            protocol = request_parts[2] if len(request_parts) > 2 else ""
            
            entry = {
                "timestamp": normalize_timestamp(timestamp),
                "level": "INFO" if int(status) < 400 else "WARN" if int(status) < 500 else "ERROR",
                "message": f"{method} {path} - {status}",
                "metadata": {
                    "ip_address": ip,
                    "user": user1 if user1 != "-" else None,
                    "http_method": method,
                    "http_path": path,
                    "http_status": status,
                    "http_version": protocol,
                    "response_size": size,
                    "referer": referer if referer != "-" else None,
                    "user_agent": user_agent if user_agent != "-" else None,
                },
                "rawData": line
            }
            entries.append(entry)
        else:
            # Fallback for non-matching lines
            entry = {
                "timestamp": None,
                "level": "INFO",
                "message": line,
                "metadata": {},
                "rawData": line
            }
            entries.append(entry)
    
    return entries

@app.post("/parse")
async def parse_log(file: UploadFile = File(...), log_format: str = Form(None)):
    try:
        with tempfile.TemporaryDirectory() as tmpdir:
            file_path = os.path.join(tmpdir, file.filename)
            with open(file_path, "wb") as f:
                shutil.copyfileobj(file.file, f)
            
            # Read all lines
            lines = []
            with open(file_path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if line:
                        lines.append(line)
            
            if not lines:
                raise HTTPException(status_code=400, detail="Empty log file")
            
            # Try hybrid JSON parsing first
            parsed_entries, unstructured_lines = parse_hybrid_json(lines)
            
            # If we have JSON entries, return them (even if mixed with unstructured)
            if parsed_entries:
                print(f"[DEBUG] Returning {len(parsed_entries)} JSON entries")
                return JSONResponse(content=parsed_entries)
            
            # Check if this looks like Apache/Nginx access logs
            if not log_format:
                log_format = detect_log_format(lines)
            
            if log_format == 'apache_nginx':
                print(f"[DEBUG] Using Apache/Nginx parser for {len(lines)} lines")
                entries = parse_apache_nginx_logs(lines)
                return JSONResponse(content=entries)
            
            # If no JSON entries, use enhanced ML parsing for unstructured logs
            print(f"[DEBUG] Using enhanced ML parser for {len(unstructured_lines)} unstructured lines")
            
            try:
                # Use enhanced ML parser with intelligent algorithm selection
                ml_entries = ml_parser.parse_logs_intelligently(unstructured_lines)
                
                if ml_entries:
                    print(f"[DEBUG] Enhanced ML parsing produced {len(ml_entries)} entries")
                    return JSONResponse(content=ml_entries)
                else:
                    # ML parsing failed, use regex fallback
                    print("[DEBUG] Enhanced ML parsing failed, using regex fallback")
                    fallback_entries = []
                    for line in unstructured_lines:
                        entry = extract_structured_fields_from_ml(content=line)
                        fallback_entries.append(entry)
                    return JSONResponse(content=fallback_entries)
                    
            except Exception as ml_error:
                print(f"[DEBUG] Enhanced ML parsing error: {ml_error}, using regex fallback")
                # Fallback to regex parsing
                fallback_entries = []
                for line in unstructured_lines:
                    entry = extract_structured_fields_from_ml(content=line)
                    fallback_entries.append(entry)
                return JSONResponse(content=fallback_entries)
            
    except Exception as e:
        print(f"[DEBUG] Exception in /parse: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "logparser"} 