#!/usr/bin/env bash
# Sorrel at a big team's size: 40 repos and 120 capsules, for measuring what a read costs.
#
# What ws's pages cost grows with the repos and capsules a workspace has
# (git per capsule, a ground per repo), and a team's workspace has many
# more than marketing's three: 40 repos and 30 to 120 capsules is a
# real one's. This adds them to marketing, so its missions, documents
# and pull requests are there too:
#
#   WS_MEASURE=1 scripts/try.sh up marketing scale
#
# It comes after marketing (it adds to Sorrel's workspace). The repos are
# svc-01 to svc-37, each about 150 files in a dozen folders with a
# fortnight's history on main. Capsules are spread over every repo, api
# and web having the most; of them, about half have commits main hasn't,
# a third changes not committed, and a quarter an open pull request on
# the file that answers for GitHub.
#
# SCALE_REPOS and SCALE_CAPSULES (40 and 120) say how many in all,
# marketing's own counted.
set -euo pipefail

dir="$TRY_DIR"
ws="$TRY_WS"
repos_in_all="${SCALE_REPOS:-40}"
capsules_in_all="${SCALE_CAPSULES:-120}"

if [ ! -d "$dir/repos/api" ]; then
	echo "scale adds to marketing: seed it first (scripts/try.sh up marketing scale)" >&2
	exit 1
fi

git_as() { # <author> <when> <git args...>
	local author="$1" when="$2"
	shift 2
	GIT_AUTHOR_DATE="$when" GIT_COMMITTER_DATE="$when" git -c user.name="$author" -c user.email="$author@example.com" -c commit.gpgsign=false "$@"
}
day() { date -u -v-"$1"d +%Y-%m-%dT10:00:00Z 2>/dev/null || date -u -d "$1 days ago" +%Y-%m-%dT10:00:00Z; }

# files <dir> <seed>: about 150 files of Go and TypeScript, in a dozen
# folders, a little different for each seed.
files() {
	python3 - "$1" "$2" <<'PY'
import os, sys, random
root, seed = sys.argv[1], int(sys.argv[2])
rng = random.Random(seed)
words = "invoice payment customer ledger rate currency webhook export report studio account tax refund".split()
folders = ["cmd/server", "internal/api", "internal/store", "internal/billing", "internal/tax", "internal/webhooks",
           "web/src/pages", "web/src/parts", "web/src/lib", "docs", "scripts", "test"]
for folder in folders:
    os.makedirs(os.path.join(root, folder), exist_ok=True)
    for i in range(rng.randint(9, 16)):
        noun = rng.choice(words)
        if folder.startswith("web"):
            name, body = f"{noun}{i}.tsx", "".join(
                f"export function {noun.title()}{i}_{n}(props: {{ id: string }}) {{\n  return <div data-id={{props.id}}>{noun} {n}</div>;\n}}\n\n"
                for n in range(rng.randint(3, 12)))
        elif folder in ("docs",):
            name, body = f"{noun}-{i}.md", f"# {noun.title()} {i}\n\n" + ("Lorem ipsum about the " + noun + ".\n") * rng.randint(5, 40)
        else:
            pkg = folder.split("/")[-1]
            name, body = f"{noun}_{i}.go", f"package {pkg}\n\n" + "".join(
                f"// {noun.title()}{n} does {noun} things.\nfunc {noun.title()}{n}(x int) int {{\n\treturn x + {n}\n}}\n\n"
                for n in range(rng.randint(3, 15)))
        with open(os.path.join(root, folder, name), "w") as f:
            f.write(body)
with open(os.path.join(root, "README.md"), "w") as f:
    f.write(f"# Service {seed}\n\nOne of Sorrel's services.\n")
PY
}

# --- the repos ------------------------------------------------------------

have=$(ls -d "$dir"/repos/*/ | wc -l | tr -d ' ')
added=()
for ((i = 1; have + i <= repos_in_all; i++)); do
	name=$(printf "svc-%02d" "$i")
	src="$dir/.remotes/$name"
	mkdir -p "$src"
	git -C "$src" init -q --initial-branch=main
	git -C "$src" config uploadpack.allowAnySHA1InWant true
	files "$src" "$i"
	git -C "$src" add -A
	git_as maya "$(day 14)" -C "$src" commit -q -m "Start $name"
	# A fortnight's history: a file or two changed a day.
	for d in 13 11 9 7 5 3 2 1; do
		f=$(git -C "$src" ls-files | sed -n "$(((i * d) % 100 + 1))p")
		echo "// changed $d days ago" >>"$src/$f"
		git -C "$src" add -A
		git_as tomas "$(day "$d")" -C "$src" commit -q -m "Tidy $(basename "$f")"
	done
	printf '\n[repos.%s]\n' "$name" >>"$dir/ws.toml"
	added+=("$name")
done
git -C "$dir" add ws.toml
git_as maya "$(day 14)" -C "$dir" commit -q -m "Sorrel's services"

for name in "${added[@]}"; do
	repo="$dir/repos/$name"
	mkdir -p "$repo"
	git clone -q --bare "$dir/.remotes/$name" "$repo/.bare"
	git -C "$repo/.bare" config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'
	git -C "$repo/.bare" fetch -q --all
	git -C "$repo/.bare" worktree add -q "$repo/.ground" main
done

# --- the capsules -----------------------------------------------------------

all=(api web records "${added[@]}")
capsules=$(find "$dir"/repos/*/ -mindepth 1 -maxdepth 1 -type d ! -name '.*' | wc -l | tr -d ' ')
prs=() # repo/branch, for "GitHub"
n=0
while ((capsules < capsules_in_all)); do
	# api and web get one in four; the rest go round the others.
	case $((n % 8)) in
	0) repo=api ;;
	1) repo=web ;;
	*) repo="${all[$((2 + n % (${#all[@]} - 2)))]}" ;;
	esac
	name=$(printf "work-%03d" "$n")
	n=$((n + 1))
	"$ws" lift "$repo" "$name" >/dev/null 2>&1 || continue
	capsules=$((capsules + 1))
	cap="$dir/repos/$repo/$name"
	branch=$(git -C "$cap" rev-parse --abbrev-ref HEAD)
	f=$(git -C "$cap" ls-files | grep -E '\.(go|tsx?|md)$' | awk -v n="$n" '{ a[NR] = $0 } END { print a[n % NR + 1] }')
	if ((n % 2 == 0)); then # commits main hasn't
		for k in 1 2 3; do
			echo "// $name, commit $k" >>"$cap/$f"
			git_as you "$(day 1)" -C "$cap" commit -q -am "$name: step $k"
		done
		if ((n % 4 == 0)); then
			prs+=("$repo $branch $(git -C "$cap" rev-parse HEAD) $n")
		fi
	fi
	if ((n % 3 == 0)); then # changes not committed
		echo "// $name, not committed" >>"$cap/$f"
		echo "new in $name" >"$cap/NOTES-$name.md"
	fi
done

# --- "GitHub": an open pull request for a quarter of them -------------------

python3 - "$dir/.remotes/github.json" "${prs[@]}" <<'PY'
import json, sys
path, prs = sys.argv[1], sys.argv[2:]
data = json.load(open(path))
repos = data.setdefault("repos", {})
for i, line in enumerate(prs):
    repo, branch, sha, n = line.split()
    entry = repos.setdefault(repo, {})
    entry.setdefault("prs", []).append({
        "number": 500 + i, "title": f"Work {n} on {repo}", "authorLogin": "you",
        "headRefName": branch, "baseRefName": "main", "headRefOid": sha,
        "url": f"https://github.com/example-org/{repo}/pull/{500 + i}",
        "statusRollup": "success", "reviewDecision": "REVIEW_REQUIRED",
        "additions": 3, "deletions": 0, "changedFiles": 1,
    })
json.dump(data, open(path, "w"), indent=2)
PY

echo "scale: ${#all[@]} repos, $capsules capsules, ${#prs[@]} more open pull requests"
