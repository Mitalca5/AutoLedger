## Summary

<!-- What changes and why. Link the issue it closes (Closes #123). -->

## Type

<!-- Keep one: bug fix / new feature / documentation / refactoring / tests / chore -->

## Notes

<!-- How you tested it, screenshots for UI changes, anything a reviewer should look at first. -->

## Checklist

- [ ] `go build ./... && go vet ./...`, `gofmt -l .` and `go test ./...` pass (backend)
- [ ] `npm run typecheck`, `npm test` and `npm run build` pass (frontend)
- [ ] New behaviour has a test next to it
- [ ] User-visible strings are in both `en` and `fr` catalogs
- [ ] No hard-coded brand, model, currency or distance unit
- [ ] Documentation updated when behaviour changes
