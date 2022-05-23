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
    my $dst = "$cachedir/content";
    my $tmp = "$cachedir/content.new";

    Storable::nstore($r, $tmp);
    rename($tmp, $dst) || die("500 rename $tmp -> $dst: $!\n");
  };
  if ($@) {
    return 1;
  }

  return 0;
}

exit(1) if @ARGV != 2;

$c = store($ARGV[0], $ARGV[1]);
exit($c);
