import subprocess
import logging

from certbot import errors
from certbot.plugins import dns_common

logger = logging.getLogger(__name__)

# =====
class Authenticator(dns_common.DNSAuthenticator):
    """ DNS Authenticator for PiKVM Cloud """

    description = "Obtain certificates using a DNS TXT record"

    def more_info(self) -> str:
        return "This plugins configures a DNS TXT record to respond to a dns-01 challenge on PiKVM Cloud"

    def _setup_credentials(self) -> None:
        pass

    def _perform(self, domain: str, validation_name: str, validation: str) -> None:
        combined_domain = f"{validation_name}.{domain}"
        res = subprocess.run(["kvmd-cloudctl", "certbotAdd", combined_domain, validation], capture_output=True)
        if res.returncode != 0:
            raise errors.PluginError(f"Error adding TXT record: {res.stderr.decode()}")
        logger.debug("Successfully added TXT record")


    def _cleanup(self, domain: str, validation_name: str, validation: str) -> None:
        combined_domain = f"{validation_name}.{domain}"
        res = subprocess.run(["kvmd-cloudctl", "certbotDel", combined_domain], capture_output=True)
        if res.returncode != 0:
            logger.warning(f"Error deleting TXT record {combined_domain}")
        else:
            logger.debug("Successfully deleted TXT record")
