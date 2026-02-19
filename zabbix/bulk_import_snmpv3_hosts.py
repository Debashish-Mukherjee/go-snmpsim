#!/usr/bin/env python3
"""
Bulk SNMP Host Configuration Script - Add 100 SNMPv3 hosts to Zabbix
"""

import requests
import json
import time
import sys
import argparse
from typing import Optional, Dict, Any, List

class ZabbixBulkHostManager:
    """Manage bulk SNMP host creation in Zabbix"""
    
    def __init__(self, url: str, username: str = "Admin", password: str = "zabbix"):
        self.url = url
        self.username = username
        self.password = password
        self.auth_token = None
        self.session = requests.Session()
        
    def request(self, method: str, params: Dict[str, Any]) -> Dict[str, Any]:
        """Make a JSON-RPC request to Zabbix API"""
        payload = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": 1
        }

        headers = {"Content-Type": "application/json"}
        if self.auth_token and method != "user.login":
            headers["Authorization"] = f"Bearer {self.auth_token}"
        
        try:
            response = self.session.post(
                f"{self.url}/api_jsonrpc.php",
                json=payload,
                headers=headers,
                timeout=30
            )
            response.raise_for_status()
            result = response.json()
            
            if "error" in result:
                raise Exception(f"Zabbix API error: {result['error']}")
            
            return result.get("result", {})
        except Exception as e:
            print(f"❌ Request failed: {e}")
            raise
    
    def login(self) -> bool:
        """Authenticate with Zabbix"""
        try:
            print(f"🔐 Logging in to Zabbix at {self.url}...")
            result = self.request("user.login", {
                "username": self.username,
                "password": self.password
            })
            self.auth_token = result
            print("✓ Authentication successful")
            return True
        except Exception as e:
            print(f"❌ Login failed: {e}")
            return False
    
    def get_hostgroup(self, name: str) -> Optional[str]:
        """Get or create a host group"""
        try:
            # Try to find existing group
            result = self.request("hostgroup.get", {
                "filter": {"name": name},
                "limit": 1
            })
            
            if result:
                return result[0]["groupid"]
            
            # Create new group
            print(f"Creating host group: {name}...")
            result = self.request("hostgroup.create", {"name": name})
            return result["groupids"][0]
        except Exception as e:
            print(f"❌ Failed to get/create hostgroup: {e}")
            raise
    
    def get_or_create_snmpv3_interface(self) -> Dict[str, Any]:
        """Return SNMPv3 interface configuration"""
        return {
            "type": 2,  # SNMP
            "main": 1,
            "useip": 0,
            "ip": "",
            "dns": "",  # Will be set per host
            "port": "161",
            "details": {
                "version": 3,
                "securityname": "simuser",
                "securitylevel": 2,  # authPriv
                "authprotocol": 1,  # SHA
                "authpassphrase": "authpass1234",
                "privprotocol": 1,  # AES
                "privpassphrase": "privpass1234",
                "contextname": ""
            }
        }
    
    def add_hosts(self, num_hosts: int = 100, start_port: int = 10000, 
                  base_ip: str = "snmpsim", hostgroup_name: str = "SNMP Simulators") -> bool:
        """Add bulk hosts to Zabbix"""
        try:
            # Get host group
            groupid = self.get_hostgroup(hostgroup_name)
            print(f"✓ Using host group ID: {groupid}")
            
            # Get available templates
            print("📋 Fetching available templates...")
            templates = self.request("template.get", {
                "output": ["templateid", "name"]
            })

            snmp_templates = [t for t in templates if "snmp" in t["name"].lower()]
            preferred = [t for t in snmp_templates if "network generic device by snmp" in t["name"].lower()]
            if preferred:
                template_id = preferred[0]["templateid"]
            elif snmp_templates:
                template_id = snmp_templates[0]["templateid"]
            else:
                template_id = None

            if template_id:
                print(f"✓ Using template ID: {template_id}")
            else:
                print("⚠️ No SNMP template found, creating hosts without templates")
            
            # Prepare hosts for creation
            hosts_data = []
            for i in range(num_hosts):
                port = start_port + i
                host_name = f"snmpsim-host-{i:03d}"
                
                interface = self.get_or_create_snmpv3_interface()
                interface["dns"] = base_ip
                interface["port"] = str(port)
                
                host = {
                    "host": host_name,
                    "name": f"SNMP Simulator Host {i:03d}",
                    "groups": [{"groupid": groupid}],
                    "interfaces": [interface],
                    "status": 0  # Monitored
                }
                
                if template_id:
                    host["templates"] = [{"templateid": template_id}]
                
                hosts_data.append(host)
            
            # Create hosts one-by-one for broad Zabbix compatibility
            created = 0
            skipped = 0
            failed = 0

            for host in hosts_data:
                try:
                    existing = self.request("host.get", {
                        "output": ["hostid"],
                        "filter": {"host": [host["host"]]},
                        "limit": 1
                    })
                    if existing:
                        skipped += 1
                        continue

                    self.request("host.create", host)
                    created += 1
                except Exception as e:
                    failed += 1
                    print(f"⚠️  Failed to create {host['host']}: {e}")

                if (created + skipped + failed) % 10 == 0:
                    print(f"Progress: {created + skipped + failed}/{num_hosts} (created={created}, skipped={skipped}, failed={failed})")
                time.sleep(0.1)
            
            print(f"\n✅ Bulk host creation completed! created={created}, skipped={skipped}, failed={failed}")
            return True
        
        except Exception as e:
            print(f"❌ Host creation failed: {e}")
            return False
    
    def get_template(self, name: str) -> Optional[str]:
        """Get template by name"""
        try:
            result = self.request("template.get", {
                "filter": {"name": name},
                "limit": 1
            })
            return result[0]["templateid"] if result else None
        except:
            return None
    
    def verify_hosts(self, hostgroup_name: str = "SNMP Simulators") -> bool:
        """Verify created hosts"""
        try:
            print("\n📊 Verifying hosts...")
            groupid = self.get_hostgroup(hostgroup_name)
            result = self.request("host.get", {
                "output": ["hostid", "host", "name"],
                "selectGroups": "extend",
                "groupids": [groupid],
                "sortfield": "host"
            })
            
            print(f"✓ Found {len(result)} hosts in '{hostgroup_name}'")
            for host in result[:5]:
                print(f"  - {host['host']} ({host['name']})")
            
            if len(result) > 5:
                print(f"  ... and {len(result) - 5} more hosts")
            
            return len(result) > 0
        except Exception as e:
            print(f"❌ Verification failed: {e}")
            return False


def main():
    parser = argparse.ArgumentParser(
        description="Bulk configure SNMP hosts in Zabbix"
    )
    parser.add_argument(
        "--url",
        default="http://localhost:8081",
        help="Zabbix web URL (default: http://localhost:8081)"
    )
    parser.add_argument(
        "--username",
        default="Admin",
        help="Zabbix admin username (default: Admin)"
    )
    parser.add_argument(
        "--password",
        default="zabbix",
        help="Zabbix admin password (default: zabbix)"
    )
    parser.add_argument(
        "--num-hosts",
        type=int,
        default=100,
        help="Number of hosts to create (default: 100)"
    )
    parser.add_argument(
        "--start-port",
        type=int,
        default=10000,
        help="Starting SNMP port (default: 10000)"
    )
    parser.add_argument(
        "--base-ip",
        default="snmpsim",
        help="Base IP/hostname for simulator (default: snmpsim)"
    )
    parser.add_argument(
        "--hostgroup",
        default="SNMP Simulators",
        help="Host group name (default: SNMP Simulators)"
    )
    parser.add_argument(
        "--verify-only",
        action="store_true",
        help="Only verify existing hosts, don't create new ones"
    )
    
    args = parser.parse_args()
    
    manager = ZabbixBulkHostManager(
        url=args.url,
        username=args.username,
        password=args.password
    )
    
    # Attempt login with retry
    max_retries = 5
    for attempt in range(max_retries):
        if manager.login():
            break
        if attempt < max_retries - 1:
            print(f"⏳ Retrying in 5 seconds... (attempt {attempt + 1}/{max_retries})")
            time.sleep(5)
        else:
            print("❌ Failed to authenticate after retries")
            sys.exit(1)
    
    if args.verify_only:
        manager.verify_hosts(args.hostgroup)
    else:
        success = manager.add_hosts(
            num_hosts=args.num_hosts,
            start_port=args.start_port,
            base_ip=args.base_ip,
            hostgroup_name=args.hostgroup
        )
        
        if success:
            print("\n✓ Waiting 10 seconds before verification...")
            time.sleep(10)
            manager.verify_hosts(args.hostgroup)
        else:
            sys.exit(1)


if __name__ == "__main__":
    main()
