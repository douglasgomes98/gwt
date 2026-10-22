class Gwt < Formula
  desc "Manage Git worktrees across sibling repositories"
  homepage "https://github.com/douglasgomes98/gwt"
  url "https://github.com/douglasgomes98/gwt/archive/refs/tags/v1.11.2.tar.gz"
  sha256 "04308fe6722ff47242987df8182318bb262e36fbc5229118aafcd1322133db7a"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/gwt"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/gwt version")
  end
end
