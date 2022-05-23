#
# Copyright (c) 2006, 2007 Michael Schroeder, Novell Inc.
# Copyright (c) 2016  Frank Schreiner, SUSE LLC
#
# This program is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License version 2 as
# published by the Free Software Foundation.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program (see the file COPYING); if not, write to the
# Free Software Foundation, Inc.,
# 51 Franklin Street, Fifth Floor, Boston, MA 02110-1301, USA
#
################################################################

package BSUtil;

=head1 NAME

BSUtil - collection of useful functions

=cut

require Exporter;
@ISA = qw(Exporter);
@EXPORT = qw{readstr};

use Storable ();
use IO::Handle;
use strict;

sub readstr {
  my ($fn, $nonfatal) = @_;
  my $f;
  if (!open($f, '<', $fn)) {
    die("$fn: $!\n") unless $nonfatal;
    return undef;
  }
  my $d = '';
  1 while sysread($f, $d, 8192, length($d));
  close $f;
  return $d;
}

sub retrieve {
  my ($fn, $nonfatal) = @_;
  my $dd;
  if (!$nonfatal) {
    $dd = ref($fn) ? Storable::fd_retrieve($fn) : Storable::retrieve($fn);
    die("retrieve $fn: $!\n") unless $dd;
  } else {
    eval {
      $dd = ref($fn) ? Storable::fd_retrieve($fn) : Storable::retrieve($fn);
    };
    if (!$dd && $nonfatal == 2) {
      if ($@) {
        warn($@);
      } else {
        warn("retrieve $fn: $!\n");
      }
    }
  }
  return $dd;
}

sub fromstorable {
  my $nonfatal = $_[1];
  return Storable::thaw(substr($_[0], 4)) unless $nonfatal;
  my $d;
  eval { $d = Storable::thaw(substr($_[0], 4)); };
  if ($@) {
    warn($@) if $nonfatal == 2;
    return undef;
  }
  return $d;
}
