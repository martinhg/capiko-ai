package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// capyBrown is the warm brand brown the capybara mascot is drawn in. Under a
// non-color profile it collapses to plain text, so golden snapshots stay
// deterministic.
var capyBrown = lipgloss.NewStyle().Foreground(lipgloss.Color("#A0703B"))

// capyArt is the capiko capybara mascot in ASCII shading: 56 columns wide and
// 26 rows tall. The main menu box is sized to hold it. Leading spaces are
// significant — they place the shape — so the literal must not be reflowed. The
// leading newline keeps the block readable and is trimmed before rendering.
const capyArt = `
                               %=#
                 %=+==   *%%%%%%%%
                 %+++#==================%:
                   %#=====+==*=========+++++%
                  +===================++++++%%
                 %===================++++++++%
                #====================++++++++%
                #====================+++++*++-
               %=====================#+++%=%%
             ============================:%
           %=================+=========%= :%  :*+=++
         %====================+++++++     :: +:%==:=:=
       %=========================++++#     :     #+++%%
      ===============================%     ::   %
     ================================%   % +=   #  = +
    +================================: @    %::--   %%
   %=================================:*****#:  =#****#%
   +===========+====================+:+#*************##
  -++=============#==========%======#-*****#**********#
  -++==============+=========%======  *=*************#%
   +++=================+=====%=%===%  %+************##:
   %+++=============*=+++====+  +++#   %********#**##%
    ++++============++++++++++#:#++%#:-:%##*****####
:::::%++++========#%%%%:-::++++%%++++%-::::%####%=::::::
:::::::-%++++====+++++++*-:-++*+*%::::::::::::::::::::::
         :-::--::--:-%%%::::::::::::::::::---::`

// logo renders the capybara mascot in the warm brand brown, one line at a time
// so each row keeps its own width inside the menu box.
func logo() string {
	lines := strings.Split(strings.Trim(capyArt, "\n"), "\n")
	for i, l := range lines {
		lines[i] = capyBrown.Render(l)
	}
	return strings.Join(lines, "\n")
}
