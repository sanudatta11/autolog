import requests
import tempfile
import os
import json
import datetime

def test_parse_json_logs():
    # Sample JSON log lines
    log_lines = [
        {"timestamp": "2025-07-04T10:00:00Z", "level": "INFO", "message": "App starting", "metadata": {"foo": "bar"}},
        {"timestamp": "2025-07-04T10:00:01Z", "level": "ERROR", "message": "Something failed", "metadata": {"error_code": 123}},
    ]
    log_file_content = "\n".join(json.dumps(line) for line in log_lines)

    # Write to a temp file
    with tempfile.NamedTemporaryFile(delete=False, mode="w", suffix=".log") as tmp:
        tmp.write(log_file_content)
        tmp_path = tmp.name

    # Send to microservice
    url = "http://localhost:8000/parse"
    with open(tmp_path, "rb") as f:
        files = {"file": (os.path.basename(tmp_path), f)}
        response = requests.post(url, files=files)
    os.remove(tmp_path)

    assert response.status_code == 200, f"Status code: {response.status_code}, body: {response.text}"
    data = response.json()
    print("Response data:", data)
    assert isinstance(data, list) and len(data) == 2
    for i, entry in enumerate(data):
        if "timestamp" not in entry:
            print(f"[TEST WARNING] 'timestamp' missing in entry {i}: {entry}")
        assert entry.get("timestamp") == log_lines[i]["timestamp"], f"Expected timestamp {log_lines[i]['timestamp']}, got {entry.get('timestamp')}"
        assert entry.get("level") == log_lines[i]["level"], f"Expected level {log_lines[i]['level']}, got {entry.get('level')}"
        assert entry.get("message") == log_lines[i]["message"], f"Expected message {log_lines[i]['message']}, got {entry.get('message')}"
        assert entry.get("metadata") == log_lines[i]["metadata"], f"Expected metadata {log_lines[i]['metadata']}, got {entry.get('metadata')}"
        assert "rawData" in entry

if __name__ == "__main__":
    test_parse_json_logs()
    print("Test passed!") 