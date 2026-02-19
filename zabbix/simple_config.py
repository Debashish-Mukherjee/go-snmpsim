#!/usr/bin/env python3
"""
Simple Zabbix API test and SNMPv3 host configuration
"""

import requests
import json
import sys

# Colors for output
GREEN = '\033[92m'
RED = '\033[91m'
YELLOW = '\033[93m'
RESET = '\033[0m'

def api_call(url, method, params, auth=None):
    """Make JSON-RPC API call to Zabbix"""
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": 1
    }
    
    # Auth should NOT be in params, it should be at the root level
    if auth:
        payload["auth"] = auth
    
    print(f"\n→ Calling {method}...")
    print(f"  Payload keys: {list(payload.keys())}")
    try:
        response = requests.post(
            f"{url}/api_jsonrpc.php",
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=10
        )
        result = response.json()
        
        if "error" in result:
            error = result['error']
            print(f"{RED}❌ Error: {error}{RESET}")
            return None
        
        print(f"{GREEN}✓ Success{RESET}")
        return result.get("result")
    except Exception as e:
        print(f"{RED}❌ Request failed: {e}{RESET}")
        return None

def main():
    zabbix_url = "http://localhost:8081"
    
    print(f"{YELLOW}╔═══════════════════════════════════════════╗{RESET}")
    print(f"{YELLOW}║  Zabbix API Configuration for SNMP Hosts  ║{RESET}")
    print(f"{YELLOW}╚═══════════════════════════════════════════╝{RESET}")
    
    # Step 1: Login
    print(f"\n{YELLOW}[1/4] Authenticating with Zabbix...{RESET}")
    auth_result = api_call(
        zabbix_url,
        "user.login",
        {
            "username": "Admin",
            "password": "zabbix"
        }
    )
    
    if not auth_result:
        print(f"{RED}Failed to authenticate!{RESET}")
        sys.exit(1)
    
    auth_token = auth_result
    print(f"Auth token: {auth_token[:20]}...")
    
    # Step 2: Get or create host group
    print(f"\n{YELLOW}[2/4] Setting up host group...{RESET}")
    groups = api_call(
        zabbix_url,
        "hostgroup.get",
        {
            "filter": {"name": "SNMP Simulators"},
            "limit": 1
        },
        auth_token
    )
    
    if groups:
        group_id = groups[0]["groupid"]
        print(f"Found existing group: {group_id}")
    else:
        print("Creating new host group...")
        result = api_call(
            zabbix_url,
            "hostgroup.create",
            {"name": "SNMP Simulators"},
            auth_token
        )
        if result and "groupids" in result:
            group_id = result["groupids"][0]
            print(f"Created group: {group_id}")
        else:
            print(f"{RED}Failed to create group!{RESET}")
            sys.exit(1)
    
    # Step 3: Check for templates
    print(f"\n{YELLOW}[3/4] Checking templates...{RESET}")
    templates = api_call(
        zabbix_url,
        "template.get",
        {
            "output": ["templateid", "name"],
            "limit": 5
        },
        auth_token
    )
    
    if templates:
        print(f"Found {len(templates)} templates:")
        for t in templates:
            print(f"  - {t['name']} ({t['templateid']})")
        template_id = templates[0]["templateid"]
    else:
        template_id = None
        print("No templates found (optional)")
    
    # Step 4: Add SNMPv3 test host
    print(f"\n{YELLOW}[4/4] Creating SNMP hosts...{RESET}")
    
    created_count = 0
    failed_count = 0
    
    # Create first 5 hosts as test
    for i in range(5):
        port = 10000 + i
        host_name = f"snmpsim-host-{i:03d}"
        
        interface = {
            "type": 3,  # SNMP
            "main": 1,
            "useip": 1,
            "ip": "snmpsim",
            "dns": "",
            "port": str(port),
            "details": {
                "version": 3,
                "securityname": "simuser",
                "securitylevel": 3,
                "authprotocol": 2,  # SHA
                "authpassphrase": "authpass1234",
                "privprotocol": 1,  # AES
                "privpassphrase": "privpass1234",
                "contextname": ""
            }
        }
        
        host_data = {
            "host": host_name,
            "name": f"SNMP Simulator Host {i:03d}",
            "groups": [{"groupid": group_id}],
            "interfaces": [interface],
            "status": 0  # Monitored
        }
        
        if template_id:
            host_data["templates"] = [{"templateid": template_id}]
        
        result = api_call(
            zabbix_url,
            "host.create",
            host_data,
            auth_token
        )
        
        if result and "hostids" in result:
            print(f"  ✓ Created host {i+1}/5: {host_name}")
            created_count += 1
        else:
            print(f"  ✗ Failed to create host {i+1}/5: {host_name}")
            failed_count += 1
    
    # Summary
    print(f"\n{YELLOW}═══════════════════════════════════════════{RESET}")
    print(f"{GREEN}✓ Created: {created_count} hosts{RESET}")
    if failed_count > 0:
        print(f"{RED}✗ Failed: {failed_count} hosts{RESET}")
    print(f"{YELLOW}═══════════════════════════════════════════{RESET}")
    
    if created_count > 0:
        print(f"\n{GREEN}✓ SUCCESS! Hosts are now configured in Zabbix{RESET}")
        print(f"\nNext steps:")
        print(f"  1. Open http://localhost:8081 (Zabbix Web)")
        print(f"  2. Go to Configuration → Hosts")
        print(f"  3. Find 'SNMP Simulators' group")
        print(f"  4. Hosts should appear in ~30 seconds")
        print(f"  5. Check Monitoring → Latest Data for metrics")

if __name__ == "__main__":
    main()
