#!/usr/bin/env python3
"""
Zabbix API Client Wrapper - Handles authentication and common operations
"""

import requests
import json
import time
from typing import Dict, Any, List, Optional
from urllib.parse import urljoin


class ZabbixAPIError(Exception):
    """Zabbix API Error Exception"""
    pass


class ZabbixAPIClient:
    """
    Reusable Zabbix API client for host/item management
    Handles authentication, retries, and common operations
    """

    def __init__(self, url: str, username: str = "Admin", password: str = "zabbix"):
        """
        Initialize Zabbix API client
        
        Args:
            url: Zabbix server URL (e.g., http://localhost:8081)
            username: API username (default: Admin)
            password: API password (default: zabbix)
        """
        self.url = url
        self.username = username
        self.password = password
        self.auth_token = None
        self.request_id = 0
        
    def _request(self, method: str, params: Dict[str, Any] = None) -> Dict[str, Any]:
        """
        Make a JSON-RPC request to Zabbix API
        
        Args:
            method: Zabbix API method (e.g., "host.get", "item.create")
            params: Method parameters
            
        Returns:
            API response result
            
        Raises:
            ZabbixAPIError: If API call fails
        """
        if params is None:
            params = {}
            
        self.request_id += 1
        
        payload = {
            "jsonrpc": "2.0",
            "method": method,
            "params": params,
            "id": self.request_id
        }
        
        headers = {
            "Content-Type": "application/json"
        }
        
        # For Zabbix 7.0+, use Authorization header instead of auth token in payload
        if self.auth_token and method != "user.login":
            headers["Authorization"] = f"Bearer {self.auth_token}"
        
        api_url = urljoin(self.url, "/api_jsonrpc.php")
        
        try:
            response = requests.post(
                api_url,
                json=payload,
                headers=headers,
                timeout=30
            )
            response.raise_for_status()
            
            data = response.json()
            
            # Check for API errors
            if "error" in data:
                raise ZabbixAPIError(f"API Error: {data['error']}")
            
            # Check for RPC errors
            if data.get("error"):
                raise ZabbixAPIError(f"RPC Error: {data['error']}")
            
            return data.get("result", {})
            
        except requests.RequestException as e:
            raise ZabbixAPIError(f"Request failed: {e}")
        except json.JSONDecodeError as e:
            raise ZabbixAPIError(f"Invalid JSON response: {e}")
    
    def login(self) -> bool:
        """
        Authenticate with Zabbix API
        
        Returns:
            True if authentication successful
            
        Raises:
            ZabbixAPIError: If authentication fails
        """
        params = {
            "username": self.username,
            "password": self.password
        }
        
        try:
            result = self._request("user.login", params)
            if isinstance(result, str):
                self.auth_token = result
                return True
            else:
                raise ZabbixAPIError(f"Unexpected login response: {result}")
        except ZabbixAPIError as e:
            raise ZabbixAPIError(f"Login failed: {e}")
    
    def logout(self) -> bool:
        """
        Logout from Zabbix API
        
        Returns:
            True if logout successful
        """
        try:
            self._request("user.logout")
            self.auth_token = None
            return True
        except ZabbixAPIError:
            return False
    
    def get_host(self, filter_dict: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Get host by filter
        
        Args:
            filter_dict: Filter criteria (e.g., {"host": "hostname"})
            
        Returns:
            Host object or None if not found
        """
        params = {
            "filter": filter_dict,
            "output": "extend"
        }
        
        results = self._request("host.get", params)
        return results[0] if results else None
    
    def get_all_hosts(self) -> List[Dict[str, Any]]:
        """Get all monitored hosts"""
        params = {
            "output": ["hostid", "host", "name", "status"],
            "sortfield": "name"
        }
        
        return self._request("host.get", params)
    
    def create_host(
        self,
        hostname: str,
        ip_address: str,
        port: int = 161,
        snmp_version: str = "2",
        community: str = "public",
        group_id: str = "5"  # Changed default to "Discovered hosts"
    ) -> str:
        """
        Create a new host for SNMP monitoring
        
        Args:
            hostname: Host name (e.g., "cisco-iosxr-001")
            ip_address: Host IP address
            port: SNMP port (default: 161)
            snmp_version: SNMP version ("1" or "2" or "3")
            community: SNMP community (for SNMPv1/v2)
            group_id: Host group ID (default: 5 = "Discovered hosts")
            
        Returns:
            Host ID
            
        Raises:
            ZabbixAPIError: If host creation fails
        """
        # Convert version to integer for Zabbix 7.x
        version_int = int(snmp_version)
        
        # Create SNMP interface (Zabbix 7.x format)
        interfaces = [
            {
                "type": 2,  # SNMP interface
                "main": 1,
                "useip": 1,  # Required in Zabbix 7.x
                "ip": ip_address,
                "dns": "",
                "port": str(port),
                "details": {
                    "version": version_int,  # Integer, not string
                    "community": "{$SNMP_COMMUNITY}" if community == "public" else community
                }
            }
        ]
        
        # Host object
        host_data = {
            "host": hostname,
            "interfaces": interfaces,
            "groups": [{"groupid": group_id}],
            "status": 0  # 0 = enabled, 1 = disabled
        }
        
        try:
            result = self._request("host.create", host_data)
            hostids = result.get("hostids", [])
            if hostids:
                return hostids[0]
            else:
                raise ZabbixAPIError(f"Host creation response missing hostids: {result}")
        except ZabbixAPIError as e:
            raise ZabbixAPIError(f"Failed to create host {hostname}: {e}")
    
    def delete_host(self, hostid: str) -> bool:
        """
        Delete a host
        
        Args:
            hostid: Host ID to delete
            
        Returns:
            True if successful
        """
        params = {"hostids": [hostid]}
        result = self._request("host.delete", params)
        return bool(result.get("hostids"))
    
    def delete_host_by_name(self, hostname: str) -> bool:
        """
        Delete a host by hostname
        
        Args:
            hostname: Hostname to delete
            
        Returns:
            True if successful
        """
        host = self.get_host({"host": hostname})
        if host:
            return self.delete_host(host["hostid"])
        return False
    
    def get_host_items(self, hostid: str) -> List[Dict[str, Any]]:
        """
        Get all items for a host
        
        Args:
            hostid: Host ID
            
        Returns:
            List of item objects
        """
        params = {
            "hostids": [hostid],
            "output": ["itemid", "name", "key_", "value_type"],
            "sortfield": "name"
        }
        
        return self._request("item.get", params)
    
    def create_item(
        self,
        hostid: str,
        name: str,
        key: str,
        snmp_oid: str,
        item_type: int = 20,  # SNMP agent
        value_type: int = 4,  # Unsigned integer
        update_interval: str = "5m"
    ) -> str:
        """
        Create a monitoring item for a host
        
        Args:
            hostid: Host ID
            name: Item name (display name)
            key: Item key (internal identifier)
            snmp_oid: SNMP OID to monitor
            item_type: Item type (20 = SNMP agent)
            value_type: Value type (0=float, 1=str, 3=log, 4=uint)
            update_interval: Update interval (e.g., "30s", "5m")
            
        Returns:
            Item ID
            
        Raises:
            ZabbixAPIError: If item creation fails
        """
        item_data = {
            "hostid": hostid,
            "name": name,
            "key_": key,
            "type": item_type,
            "value_type": value_type,
            "delay": update_interval,
            "snmp_oid": snmp_oid,
            "status": 0  # 0 = enabled
        }
        
        try:
            result = self._request("item.create", item_data)
            itemids = result.get("itemids", [])
            if itemids:
                return itemids[0]
            else:
                raise ZabbixAPIError(f"Item creation response missing itemids: {result}")
        except ZabbixAPIError as e:
            raise ZabbixAPIError(f"Failed to create item {name}: {e}")
    
    def create_bulk_items(
        self,
        hostid: str,
        items: List[Dict[str, Any]],
        update_interval: str = "5m"
    ) -> List[str]:
        """
        Create multiple items for a host in bulk
        
        Args:
            hostid: Host ID
            items: List of item dicts with "name", "key", "oid", "type"
            update_interval: Update interval for all items
            
        Returns:
            List of created item IDs
        """
        item_list = []
        for item in items:
            item_data = {
                "hostid": hostid,
                "name": item.get("name"),
                "key_": item.get("key"),
                "type": 20,  # SNMP agent
                "value_type": item.get("type", 4),  # Unsigned int by default
                "delay": update_interval,
                "snmp_oid": item.get("oid"),
                "status": 0
            }
            item_list.append(item_data)
        
        try:
            result = self._request("item.create", {"items": item_list})
            return result.get("itemids", [])
        except ZabbixAPIError as e:
            raise ZabbixAPIError(f"Bulk item creation failed: {e}")
    
    def update_polling_interval(
        self,
        hostid: str,
        interval: str = "5m"
    ) -> bool:
        """
        Update polling interval for all items on a host
        
        Args:
            hostid: Host ID
            interval: New interval (e.g., "30s", "5m", "1h")
            
        Returns:
            True if successful
        """
        # Get all items for this host
        items = self.get_host_items(hostid)
        
        if not items:
            return True  # No items to update
        
        # Update each item
        for item in items:
            try:
                update_data = {
                    "itemid": item["itemid"],
                    "delay": interval
                }
                self._request("item.update", update_data)
            except ZabbixAPIError:
                return False
        
        return True
    
    def get_host_values(
        self,
        hostid: str,
        item_key: str,
        limit: int = 10
    ) -> List[Dict[str, Any]]:
        """
        Get recent values for an item
        
        Args:
            hostid: Host ID
            item_key: Item key
            limit: Number of values to retrieve
            
        Returns:
            List of value objects with timestamp
        """
        # First get the item
        params = {
            "hostids": [hostid],
            "filter": {"key_": item_key},
            "output": ["itemid"]
        }
        items = self._request("item.get", params)
        
        if not items:
            return []
        
        itemid = items[0]["itemid"]
        
        # Get history values
        params = {
            "itemids": [itemid],
            "history": 0,  # 0=float, 1=char, 2=log, 3=int, 4=text
            "limit": limit,
            "output": ["itemid", "clock", "value"],
            "sortfield": "clock",
            "sortorder": "DESC"
        }
        
        return self._request("history.get", params)
    
    def wait_for_server(self, max_retries: int = 30, retry_interval: int = 1) -> bool:
        """
        Wait for Zabbix server to become available
        
        Args:
            max_retries: Maximum number of retry attempts
            retry_interval: Seconds between retries
            
        Returns:
            True if server becomes available
        """
        for attempt in range(max_retries):
            try:
                # Try to get info (doesn't require auth)
                response = requests.get(
                    urljoin(self.url, "/api_jsonrpc.php"),
                    timeout=5
                )
                if response.status_code == 200:
                    return True
            except requests.RequestException:
                pass
            
            if attempt < max_retries - 1:
                print(f"  ⏳ Waiting for Zabbix... ({attempt + 1}/{max_retries})")
                time.sleep(retry_interval)
        
        return False

    def get_version(self) -> str:
        """Get Zabbix server version"""
        try:
            result = self._request("apiinfo.version")
            return result if isinstance(result, str) else str(result)
        except ZabbixAPIError:
            return "Unknown"
