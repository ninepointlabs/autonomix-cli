# Maintainer: Tim <tim@example.com>
pkgname=autonomix-cli
pkgver=0.2.7
pkgrel=1
pkgdesc="A CLI for managing Linux applications from GitHub releases"
arch=('x86_64' 'aarch64')
url="https://github.com/ninepointlabs/autonomix-cli"
license=('MIT')
depends=()
makedepends=('go')
source=("$pkgname-$pkgver.tar.gz::$url/archive/v$pkgver.tar.gz")
sha256sums=('SKIP')

build() {
  cd "$pkgname-$pkgver"
  export CGO_ENABLED=0
  export GOFLAGS="-buildmode=pie -trimpath -ldflags=-linkmode=external -mod=readonly -modcacherw"
  go build -o "$pkgname" .
}

package() {
  cd "$pkgname-$pkgver"
  install -Dm755 "$pkgname" "$pkgdir/usr/bin/$pkgname"
  install -Dm644 README.md "$pkgdir/usr/share/doc/$pkgname/README.md"
}
