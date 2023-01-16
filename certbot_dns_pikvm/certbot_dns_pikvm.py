import subprocess
import logging

from typing import Callable

from certbot import errors
from certbot.plugins import dns_common


# =====
_logger = logging.getLogger(__name__)


# =====
class Authenticator(dns_common.DNSAuthenticator):
    """ DNS Authenticator for PiKVM Cloud """

    description = "Obtain certificates using a DNS TXT record"

    def more_info(self) -> str:
        return "This plugins configures a DNS TXT record to respond to a dns-01 challenge on PiKVM Cloud"

    @classmethod
    def add_parser_arguments(cls, add: Callable[..., None], default_propagation_seconds: int=10) -> None:
        super().add_parser_arguments(add, default_propagation_seconds)
        add("credentials", help='Stub')

    def _setup_credentials(self) -> None:
        pass

    def _perform(self, domain: str, validation_name: str, validation: str) -> None:
        combined_domain = f"{validation_name}.{domain}"
        res = subprocess.run(["kvmd-cloudctl", "certbotAdd", combined_domain, validation], capture_output=True)
        if res.returncode != 0:
            raise errors.PluginError(f"Error adding TXT record: {res.stderr.decode()}")
        _logger.debug("Successfully added TXT record")


    def _cleanup(self, domain: str, validation_name: str, validation: str) -> None:
        combined_domain = f"{validation_name}.{domain}"
        res = subprocess.run(["kvmd-cloudctl", "certbotDel", combined_domain], capture_output=True)
        if res.returncode != 0:
            _logger.warning(f"Error deleting TXT record {combined_domain}")
        else:
            _logger.debug("Successfully deleted TXT record")
