from certbot.plugins import dns_common


# =====
class Authenticator(dns_common.DNSAuthenticator):
    """ DNS Authenticator for PiKVM Cloud """

    description = "Obtain certificates using a DNS TXT record"

    def more_info(self) -> str:
        return "This plugins configures a DNS TXT record to respond to a dns-01 challenge on PiKVM Cloud"

    def _setup_credentials(self) -> None:
        pass

    def _perform(self, domain: str, validation_name: str, validation: str) -> None:
        pass  # TODO: Add TXT record

    def _cleanup(self, domain: str, validation_name: str, validation: str) -> None:
        pass  # TODO: Remove TXT record
