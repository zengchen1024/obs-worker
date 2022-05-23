use BSUtil;

sub parse {
  my ($f) = @_;

  my $s = BSUtil::readstr($f, 2);

  my $serverimages;
  if ($s && substr($s, 0, 4) eq 'pst0') {
    $serverimages = BSUtil::fromstorable($s, 2);
  }

  return "" unless $serverimages && ref($serverimages);
 
  my @v;
  my $item;
  for my $image (@$serverimages) {
    $item =  "- prpa: ".$image->{"prpa"};
    $item .= "  file: ".$image->{"file"};
    $item .= "  path: ".$image->{"path"};
    $item .= "  package: ".$image->{"package"};
    $item .= "  sizek: ".$image->{"sizek"} if $image->{"sizek"};
    $item .= "  hdrmd5: ".$image->{"hdrmd5"} if $image->{"hdrmd5"};

    if ($image->{"hdrmd5s"}) {
      $item .= "  hdrmd5s:\n  - ".join("\n  - ", @$image->{"hdrmd5s"});
    }

    push @v, $item;
  }

  return "images:\n".join("\n", @v);
}

exit(1) if @ARGV == 0;

$c = parse($ARGV[0]);
print $c;
