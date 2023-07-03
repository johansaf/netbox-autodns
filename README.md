# netbox-autodns

Receive [Netbox webhooks](https://demo.netbox.dev/static/docs/additional-features/webhooks/) and create, update and delete forward and/or reverse DNS records using the [go-powerdns](https://github.com/joeig/go-powerdns) module.

## Requirements

- A working Netbox and PowerDNS setup
- PowerDNS must have the API enabled with an API key

This has been tested with Netbox v3.5.2 and newer.

## Configuration

These environment variables are available to configure the application, showing the default values:

```
LISTEN_ADDRESS=":8080"                   # IP and port to bind to
PDNS_API_HOST="http://dns.example.com"   # The PowerDNS API URL (required)
PDNS_API_KEY="ChangeMe"                  # PowerDNS API key (required)
DOMAIN="example.com"                     # Domain to be used for forward records (required, even if not used)
SECRET=""                                # Optional secret to verify the HMAC signature of the webhook request
SKIP_FORWARD_RECORDS=""                  # If set to anything then disable the creation of forward records
SKIP_REVERSE_RECORDS=""                  # If set to anything then disable the creation of reverse records
```

## Netbox configuration

Two webhooks are used, one for creating and updating the records and one for deleting. The create/update webhook has some conditionals set to only send requests matching certain criteria, while the delete webhook will send a request for any delete operation.

### Create/update

Something like this should work:

- Content types: IPAM > IP Address
- Events: Creations + Updated
- HTTP method: POST
- HTTP content type: application/json
- Conditions:
```json
{
	"and": [
		{
			"attr": "status.value",
			"value": "active"
		},
		{
			"attr": "vrf",
			"value": null
		},
		{
			"attr": "dns_name",
			"value": "",
			"negate": true
		}
	]
}
```

The conditions will ensure a request only is sent if the status of the IP address is active, the VRF is not set (implying the global VRF), and the DNS name field is non-empty.

If you're using a Netbox version newer than 3.5.3 you can also add a dependency on the tag "auto-dns" by adding this:
```json
{
	"op": "contains",
	"attr": "tags.name",
	"value": "auto-dns"
}
```

Now whenever an IP address is created, updated or deleted this should be reflected in the DNS zones.

### Delete

- Content types: IPAM > IP Address
- Events: Deletions
- HTTP method: POST
- HTTP content type: application/json

## Security concerns

It's possible to modify and/or delete already existing records, also those that's created outside of Netbox.

## Known issues

- If certain steps are taken the old DNS records are not removed. The steps involved is as follow:
    - Create an IP address specifying the DNS name
    - Edit the IP address and remove the DNS name
    - Edit the IP address again and set the DNS name
- All zones are presumed to be /24 for IPv4, and /32 for IPv6
