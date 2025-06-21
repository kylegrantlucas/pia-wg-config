---
name: Region/Connection Question
about: Questions about regions, connections, or "region not found" errors
title: '[REGION] '
labels: question, documentation
assignees: ''

---

## ⚠️ STOP! Read This First

**This tool supports ALL PIA regions!** You don't need to fork this repository or modify code to use different regions.

### Quick Solutions:

**To see all available regions:**
```bash
pia-wg-config regions
```

**To use a specific region:**
```bash
pia-wg-config -r REGION_NAME USERNAME PASSWORD
```

**Popular regions:**
- `uk_london` - United Kingdom
- `de_frankfurt` - Germany  
- `ca_toronto` - Canada
- `au_sydney` - Australia
- `japan` - Japan
- `netherlands` - Netherlands

---

## If you still need help after trying the above:

**What region are you trying to connect to?**
[e.g. UK, Germany, Japan, etc.]

**What command did you run?**
```bash
# Paste your exact command here
```

**What error message did you get?**
```
Paste the error message here
```

**Have you run the regions command?**
- [ ] Yes, I ran `pia-wg-config regions` and saw the list
- [ ] No, I haven't tried this yet

**What did you expect to happen?**
A clear description of what you were trying to achieve.

**Additional context**
Any other information that might help us assist you.

---

## Before submitting:
- [ ] I have run `pia-wg-config regions` to see available regions
- [ ] I have tried using the `-r` flag with different region names
- [ ] I have verified my PIA credentials work
- [ ] I understand that regions are configurable via CLI flags, not hardcoded