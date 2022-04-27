use BSUtil;

sub parse {
  my ($cachedir) = @_;
 
  my $content = BSUtil::retrieve("$cachedir/content", 1);

  if (!$content) {
    return "";
  }

  my @v;
  for my $c (@$content) {
    push @v, "- id: $c->[0]\n  size: $c->[1]";
  }

  return "content:\n".join("\n", @v);
}

exit(1) if @ARGV == 0;

$c = parse($ARGV[0]);
print $c;
