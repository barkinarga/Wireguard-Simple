# WireGuard Simple v0.4.13

Keep client creation stable when advanced settings are expanded, and keep navigation labels on one line.

- Use a fixed 640px client-dialog width, constrained to the viewport. Keep its top/left position unchanged when expanding advanced options; grow downward up to the available height, then scroll the form body while actions remain visible.
- Keep advanced fields in two columns where space permits and one column on small screens. Remove the width-changing toggle class and redundant independent-key information box.
- Give desktop/tablet navigation enough width for the complete English Storage & monitoring label, including its selected state. Keep labels on one line and provide full translated labels as hover titles if truncation is needed.

Validation: TypeScript/build, translation and client-address tests. Real-browser measurements verify identical 640px width, left and top positions before/after repeated toggles; selected navigation fits on a single line at 1440px and 1024px. At 320px, fields use one column, no horizontal page overflow occurs, and footer actions remain reachable when scrolling to the last field.
