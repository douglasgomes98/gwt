#!/bin/sh
set -eu

tag=${1:?release tag is required}
formula=Formula/gwt.rb
version=${tag#v}
url="https://github.com/douglasgomes98/gwt/archive/refs/tags/${tag}.tar.gz"
sha=$(curl --fail --location --silent --show-error "$url" | shasum -a 256 | awk '{print $1}')

goreleaser release --clean
cat >"$formula" <<EOF
class Gwt < Formula
  desc "Manage Git worktrees across sibling repositories"
  homepage "https://github.com/douglasgomes98/gwt"
  url "$url"
  sha256 "$sha"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/gwt"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gwt version")
  end
end
EOF
git add "$formula"
git -c user.name='gwt release bot' -c user.email='gwt-release@users.noreply.github.com' commit -m "chore(formula): update gwt to $tag"
git pull --rebase origin main
git push origin HEAD:main

# Legacy mirror: users who installed from douglasgomes98/tap keep receiving
# updates through "brew upgrade". Skipped when no mirror checkout is provided.
if [ -n "${TAP_DIR:-}" ]; then
	mkdir -p "$TAP_DIR/Formula"
	cp "$formula" "$TAP_DIR/Formula/gwt.rb"
	git -C "$TAP_DIR" add Formula/gwt.rb
	git -C "$TAP_DIR" -c user.name='gwt release bot' -c user.email='gwt-release@users.noreply.github.com' commit -m "Brew formula update for gwt $tag"
	git -C "$TAP_DIR" push
fi
