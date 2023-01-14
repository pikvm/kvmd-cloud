from setuptools import setup


# =====
setup(
    name="certbot-dns-pikvm",
    package="certbot_dns_pikvm.py",
    install_requires=[
        "certbot",
    ],
    entry_points={
        "certbot.plugins": [
            "dns-pikvm = certbot_dns_pikvm:Authenticator",
        ],
    },
)
