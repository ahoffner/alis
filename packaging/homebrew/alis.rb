# Template for Formula/alis.rb in github.com/ahoffner/homebrew-tap.
# The release workflow keeps url and sha256 up to date there on every v* tag.
class Alis < Formula
  desc "Shell shortcuts organised into shareable packs"
  homepage "https://github.com/ahoffner/alis"
  url "https://github.com/ahoffner/alis/archive/refs/tags/v3.0.0.tar.gz"
  sha256 "0000000000000000000000000000000000000000000000000000000000000000"
  head "https://github.com/ahoffner/alis.git", branch: "main"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X main.version=#{version}
      -X main.shareDir=#{HOMEBREW_PREFIX}/share/alis
    ]
    system "go", "build", *std_go_args(ldflags:), "./cmd/alis"
    pkgshare.install "packs", "lib"
    generate_completions_from_executable(bin/"alis", "completion")
  end

  def caveats
    <<~EOS
      To load alis in new shells, run:
        alis setup
      or add this to the end of ~/.zshrc (after oh-my-zsh, if you use it):
        eval "$(alis init zsh)"
    EOS
  end

  test do
    assert_match "gcbr", shell_output("#{bin}/alis list")
    assert_match "function gcbr", shell_output("#{bin}/alis init zsh")
    assert_empty shell_output("zsh -fc 'eval \"$(#{bin}/alis init zsh)\"' 2>&1")
  end
end
