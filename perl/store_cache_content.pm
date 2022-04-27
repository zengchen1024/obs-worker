use BSUtil;

sub store {
  my ($cachedir, $content) = @_;
 
  if (!$content) {
    return 0;
  }

  chomp $content;

  my @v = split("\n", $content);

  my @a;
  my $r = [];
  for (@v) {
    @a = split(" ", $_);
    push @$r, [$a[0], int($a[1])];
  }

  eval {
    Storable::nstore($r, "$cachedir/content.new111");
  };
  if ($@) {
    return 1;
  }

  return 0;
}

exit(1) if @ARGV != 2;

$c = store($ARGV[0], $ARGV[1]);
exit($c);
