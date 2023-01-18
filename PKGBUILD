pkgname=kvmd-cloud
pkgver=0.5
pkgrel=1
pkgdesc="PiKVM cloud agent"
url="https://github.com/pikvm/kvmd-cloud"
license=(custom)
arch=(armv7h)
depends=(kvmd)
makedepends=(go make)
install=pkg.install
source=(
	https://github.com/pikvm/kvmd-cloud/archive/v${pkgver}.tar.gz
	pkg.install
)
md5sums=(SKIP SKIP)
backup=(
	etc/kvmd/cloud/{cloud,auth}.yaml
	etc/kvmd/cloud/nginx.ctx-http.conf
)


build() {
	cd $pkgname-$pkgver
	GOMAXPROCS=1 make build VERSION=$pkgver ARCHS=arm
}

package() {
	cd $pkgname-$pkgver
	install -Dm755 -t "$pkgdir/usr/bin" bin/kvmd-*

	mkdir -p "$pkgdir/usr/lib/systemd/system"
	cp configs/kvmd-cloud.service "$pkgdir/usr/lib/systemd/system/kvmd-cloud.service"

	mkdir -p "$pkgdir/usr/lib/sysusers.d"
	cp configs/sysusers.conf "$pkgdir/usr/lib/sysusers.d/kvmd-cloud.conf"

	mkdir -p "$pkgdir/usr/share/kvmd/extras/kvmd-cloud"
	cp configs/nginx.ctx-http.share.conf "$pkgdir/usr/share/kvmd/extras/kvmd-cloud/nginx.ctx-http.conf"

	mkdir -p "$pkgdir/etc/kvmd/cloud/ssl"
	chmod 755 "$pkgdir/etc/kvmd/cloud/ssl"
	# cp configs/nginx.ctx-http.etc.conf "$pkgdir/etc/kvmd/cloud/nginx.ctx-http.conf"
}
