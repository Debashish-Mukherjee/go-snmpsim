#!/usr/bin/env python3
"""
Test script to add one device and see what works
"""

import requests
import json

url = "http://localhost:8081/api_jsonrpc.php"

# 1. Login
login_payload = {
    "jsonrpc": "2.0",
    "method": "user.login",
    "params": {
        "username": "Admin",
        "password": "zabbix"
    },
    "id": 1
}

response = requests.post(url, json=login_payload, headers={"Content-Type": "application/json"})
auth_token = response.json()["result"]
print(f"✅ Logged in, token: {auth_token[:20]}...")

# 2. Get hostgroups to see what's available
groups_payload = {
    "jsonrpc": "2.0",
    "method": "hostgroup.get",
    "params": {
        "output": ["groupid", "name"]
    },
    "id": 2
}

headers = {
    "Content-Type": "application/json",
    "Authorization": f"Bearer {auth_token}"
}

response = requests.post(url, json=groups_payload, headers=headers)
groups = response.json()["result"]
print(f"\n📁 Available host groups:")
for g in groups[:5]:
    print(f"   - {g['name']} (ID: {g['groupid']})")

# 3. Create a simple host with SNMP interface
host_payload = {
    "jsonrpc": "2.0",
    "method": "host.create",
    "params": {
        "host": "test-snmp-device-001",
        "interfaces": [
            {
                "type": 2,  # SNMP
                "main": 1,
                "useip": 1,
                "ip": "127.0.0.1",
                "dns": "",
                "port": "20000",
                "details": {
                    "version": 3,  # SNMPv3
                    "bulk": 1,
                    "securityname": "simuser",
                    "securitylevel": 0,
                    "contextname": ""
                }
            }
        ],
        "groups": [{"groupid": "2"}],  # Linux servers or discovered hosts
        "status": 0  # Enabled
    },
    "id": 3
}

print(f"\n🔨 Creating test host...")
response = requests.post(url, json=host_payload, headers=headers)
result = response.json()

if "error" in result:
    print(f"❌ Error: {result['error']}")
else:
    print(f"✅ Success! Host ID: {result['result']['hostids']}")
