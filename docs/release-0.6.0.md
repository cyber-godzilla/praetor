# 0.6.0 Release Readiness

## Character-aware display profiles (dependency satisfied)

Character-scoped content lives in a UUID-addressed profile registry. The global
`Default` profile is the fallback and template; variables, Custom Views, Action
Sets, and the selected Action Set switch together when a character becomes
active. Legacy configuration migrates into Default.

Players manage character profiles under **Esc → Character Profiles**. Each
non-default profile has an account and positive character number. After a
successful `play N` command—or `play` followed by `N`—Praetor selects the
matching profile for the logged-in account and broadcasts the switch to every
attached frontend. Character numbers therefore only need to be unique within
an account. Newly added profiles inherit Default, while inherited custom
components receive fresh UUIDs.

This supplies the authoritative account/character switch that character-aware
display profiles require. The global Default display profile already uses UUID
sidebars and ordered component-ID lists, supports any number of sidebars, and
saves drag/drop placement immediately. The next display-profile slice can now
associate layouts with character profile IDs; that association is not part of
this prerequisite.

## Wound-language catalogue (BLOCKING)

Wound highlighting must not be considered complete for 0.6.0 until all three
game-output contexts have verified fixtures and documented mappings:

1. Combat-result text when damage is applied
2. Casual character inspection (`look`)
3. Healer examination and treatment output

The combat-result catalogue is complete except for the unknown fifth-degree
burn phrase. Character-inspection logs currently verify these provisional
severity slots:

| Severity | Cutting | Bruising | Piercing | Burning |
|----------|---------|----------|----------|---------|
| 1 | `is scratched` | `is bruised` | `is punctured` | **TBD** |
| 2 | `is badly scratched` | `has a large bruise` | `is badly punctured` | **TBD** |
| 3 | `is slashed` | `has a large black bruise` | `is pierced` | **TBD** |
| 4 | `is slashed open` | `is badly bruised` | **TBD** | **TBD** |
| 5 | **TBD** | `is completely black and blue` | **TBD** | **TBD** |

These severity assignments are placeholders based on the observed wording and
ordering. Only `cut` → `is badly scratched` and `bruise` → `has a large bruise`
have clean combat-to-inspection correlations in the retained logs.

### Definition of done

- [ ] Capture or find a primary game-log/wiki reference for every **TBD** slot.
- [ ] Confirm the exact damage-severity mapping for all inspection phrases.
- [ ] Confirm fracture inspection output. An archived example suggests
      `is mangled` plus `Shards of bone penetrate the flesh of ...`, but it
      needs a repeatable or primary reference before becoming executable data.
- [ ] Catalogue healer examination strings for cuts, bruises/fractures,
      punctures, burns, bleeding, cleanliness, bandages, splints, and stitches.
- [ ] Add red-green fixtures for every confirmed phrase and important negative
      cases in each context.
- [ ] Replace provisional severity assignments with confirmed values.
- [ ] Prepare a wiki-ready summary for the maintainer to publish manually. Do
      not publish or edit the wiki from the release workflow.

Unknown placeholders are documentation only. Praetor must leave unknown text
unchanged rather than guess and produce false highlights.
