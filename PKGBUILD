pkgname=kvmd-cloud
pkgver=1.0.0
pkgrel=1
pkgdesc="PiKVM cloud agent"
url="https://github.com/pikvm/kvmd-cloud"
license=(custom)
arch=(armv7l)
depends=(kvmd)
makedepends=(go make)
install=pkg.install
source=(
	https://github.com/pikvm/kvmd-cloud/archive/v${pkgver}.tar.gz
	pkg.install
)
md5sums=(SKIP SKIP)
backup=(
	etc/kvmd/cloud/cloud.yaml
)


build() {
	cd $pkgname-$pkgver
	make build ARCHS=arm
}

package() {
	cd $pkgname-$pkgver
	install -Dm755 -t "$pkgdir/usr/bin" bin/arm/kvmd-*

	mkdir -p "$pkgdir/usr/lib/systemd/system"
	cp configs/kvmd-cloud.service "$pkgdir/usr/lib/systemd/system/kvmd-cloud.service"

	mkdir -p "$pkgdir/usr/lib/sysusers.d"
	cp configs/sysusers.conf "$pkgdir/usr/lib/sysusers.d/kvmd-cloud.conf"

	mkdir -p "$pkgdir/usr/share/kvmd/extras/kvmd-cloud"
	cp configs/nginx.ctx-http.conf "$pkgdir/usr/share/kvmd/extras/kvmd-cloud"

	mkdir -p "$pkgdir/etc/kvmd/cloud/ssl"
	chmod 755 "$pkgdir/etc/kvmd/cloud/ssl"
}
