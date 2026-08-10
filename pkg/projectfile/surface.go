// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Core 2.0: the public surface is the document backend — read, write, merge,
// the generic extension-namespace lookup, and the document model types.
// Bridge-owned extension shapes (citation, readme, forge, funding, …) and
// their accessors moved to projectfile/bridge/internal/pfmodel; bridge code
// reaches them there. Everything left here is used by ≥2 consumers or is a
// defensible generic primitive (Toggle + ParseToggle, rawdoc, …).

package projectfile

import internal "kiota.ch/projectfile/core/v2/internal/projectfile"

// Model types — aliases, so values cross the boundary with identical fields.
type (
	Person          = internal.Person
	Organization    = internal.Organization
	Identity        = internal.Identity
	Repository      = internal.Repository
	Link            = internal.Link
	License         = internal.License
	Requirements    = internal.Requirements
	Copyright       = internal.Copyright
	LocalizedString = internal.LocalizedString
	PersonConflict  = internal.PersonConflict
	Toggle          = internal.Toggle
	ReadOptions     = internal.ReadOptions
)

// Read/write + model functions — value aliases keep one implementation.
var (
	Write            = internal.Write
	ReadWithOptions  = internal.ReadWithOptions
	ReadBaseFromPath = internal.ReadBaseFromPath
	ReconcileBase    = internal.ReconcileBase

	// ImageBasename is the synthetic container-image basename rule (the single
	// home behind `projectfile get image.basename`) — ci-resolver reads it via
	// this façade instead of shelling out.
	ImageBasename = internal.ImageBasename

	MergePeople        = internal.MergePeople
	MergeOrganizations = internal.MergeOrganizations

	// Extension namespace primitives. Every bridge-side accessor in pfmodel
	// routes through LookupExtension so TOML dotted headers and flat keys
	// both resolve; SetExtension is the write-side pair. The typed shapes of
	// individual namespaces live in pfmodel, not here.
	SetExtension    = internal.SetExtension
	LookupExtension = internal.LookupExtension

	// LocalizedString + scalar-list helpers are multi-consumer (cli + bridge).
	ExtractLocalizedString        = internal.ExtractLocalizedString
	ExtractLocalizedStringForLang = internal.ExtractLocalizedStringForLang
	SetLocalizedEN                = internal.SetLocalizedEN
	AsStringList                  = internal.AsStringList

	// ParseToggle is the constructor for the Toggle type (above). Lives here
	// rather than in pfmodel because Toggle itself is core.
	ParseToggle = internal.ParseToggle
)

// Link-type, role, and repository-role string constants. Open vocabulary per
// spec, but these are the recommended set every consumer should understand.
const (
	LinkHomepage      = internal.LinkHomepage
	LinkBugs          = internal.LinkBugs
	LinkDocumentation = internal.LinkDocumentation
	LinkForum         = internal.LinkForum
	LinkChat          = internal.LinkChat
	LinkChangelog     = internal.LinkChangelog
	LinkWiki          = internal.LinkWiki
	LinkSourceCode    = internal.LinkSourceCode
	LinkFAQ           = internal.LinkFAQ

	RoleSecurity   = internal.RoleSecurity
	RoleCopyright  = internal.RoleCopyright
	RoleCommunity  = internal.RoleCommunity
	RoleMaintainer = internal.RoleMaintainer

	RepositoryRoleOrigin  = internal.RepositoryRoleOrigin
	RepositoryRoleArchive = internal.RepositoryRoleArchive
	RepositoryRoleMirror  = internal.RepositoryRoleMirror
)

// --- Phase 2 additions ---
// Symbols the command layer (the pf-bridge root + forge/scan commands) needs.
// Kept curated to real consumers.

type IncludeFailLevel = internal.IncludeFailLevel

const (
	FailOnError   = internal.FailOnError
	FailOnWarning = internal.FailOnWarning
)

var (
	ReadBase = internal.ReadBase
	// SetYAMLOutputSorted crosses the mutable toggle over the boundary — a value
	// alias would copy the underlying var. See internal/projectfile/write.go.
	SetYAMLOutputSorted = internal.SetYAMLOutputSorted
	// git-name split (spec §5.5.5), relocated here from internal/source in the
	// Phase 2 cut so core keeps the helper while source moves to the bridge.
	SplitGitName     = internal.SplitGitName
	AmbiguousGitName = internal.AmbiguousGitName
)

// --- Phase 8 additions ---
// Symbols the projectfile CLI (get/set/add/del/convert/optimize/cache) needs.

const BaseName = internal.BaseName

// RedundantInclude is one flagged entry from RedundantIncludes (below).
type RedundantInclude = internal.RedundantInclude

var (
	WriteClean              = internal.WriteClean
	ReadFromPath            = internal.ReadFromPath
	ReadFromPathWithOptions = internal.ReadFromPathWithOptions
	ReadRawWithOptions      = internal.ReadRawWithOptions
	ReadRawFromBytes        = internal.ReadRawFromBytes
	ReadRawBaseFromPath     = internal.ReadRawBaseFromPath
	FromMap                 = internal.FromMap

	// optimize
	ResolveIncludesOnly = internal.ResolveIncludesOnly
	StripRedundant      = internal.StripRedundant
	SortIncludes        = internal.SortIncludes

	// include-list hygiene (validate --strict-includes): flag a direct include
	// a sibling already provides. The list-level twin of StripRedundant.
	RedundantIncludes = internal.RedundantIncludes

	// cache warm + XDG lookup
	AllHTTPIncludes = internal.AllHTTPIncludes
	WarmInclude     = internal.WarmInclude
	XDGCacheDir     = internal.XDGCacheDir

	// Includes cache report/purge. All pf-* binaries share one slot
	// ($XDG_CACHE_HOME/pf/includes).
	IncludesCacheDir = internal.IncludesCacheDir
	PurgeIncludes    = internal.PurgeIncludes

	// YAMLOutputSortedEnabled reads the sorted-output toggle across the boundary
	// (the write is SetYAMLOutputSorted above — a mutable var cannot value-alias).
	YAMLOutputSortedEnabled = internal.YAMLOutputSortedEnabled
)
