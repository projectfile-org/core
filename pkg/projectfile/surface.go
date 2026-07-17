// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Bridge Revolution Phase 1: promote the read+write+model surface that
// internal/{bridge,forge,scanners} consume, so those packages can import this
// facade instead of internal/. Types are zero-cost aliases; functions are
// value aliases (one implementation); constants re-export the internal values.
// Curated to exactly what the movable packages reference today.

package projectfile

import internal "kiota.ch/projectfile/core/internal/projectfile"

// Model types — aliases, so values cross the boundary with identical fields.
type (
	Person                   = internal.Person
	Organization             = internal.Organization
	Identity                 = internal.Identity
	Repository               = internal.Repository
	Link                     = internal.Link
	License                  = internal.License
	Requirements             = internal.Requirements
	Dependencies             = internal.Dependencies
	Copyright                = internal.Copyright
	LocalizedString          = internal.LocalizedString
	PersonConflict           = internal.PersonConflict
	CodeOwnersEntry          = internal.CodeOwnersEntry
	VulnerabilitySuppress    = internal.VulnerabilitySuppress
	FundingChannel           = internal.FundingChannel
	FundingPlan              = internal.FundingPlan
	FundingHistory           = internal.FundingHistory
	ReleaseBranch            = internal.ReleaseBranch
	Toggle                   = internal.Toggle
	ReadOptions              = internal.ReadOptions
	IgnoreTargetOverride     = internal.IgnoreTargetOverride
	FundingExtension         = internal.FundingExtension
	SecurityExtension        = internal.SecurityExtension
	SupportExtension         = internal.SupportExtension
	ContributingExtension    = internal.ContributingExtension
	ConventionsExtension     = internal.ConventionsExtension
	IgnoresExtension         = internal.IgnoresExtension
	EditorsExtension         = internal.EditorsExtension
	VulnerabilitiesExtension = internal.VulnerabilitiesExtension
	ReadmeExtension          = internal.ReadmeExtension
	ForgeExtension           = internal.ForgeExtension
	ReleaseExtension         = internal.ReleaseExtension
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

	SetExtension = internal.SetExtension
	HasExtension = internal.HasExtension

	GetFundingExtension         = internal.GetFundingExtension
	GetSecurityExtension        = internal.GetSecurityExtension
	GetSupportExtension         = internal.GetSupportExtension
	GetContributingExtension    = internal.GetContributingExtension
	GetConventionsExtension     = internal.GetConventionsExtension
	GetIgnoresExtension         = internal.GetIgnoresExtension
	GetEditorsExtension         = internal.GetEditorsExtension
	GetVulnerabilitiesExtension = internal.GetVulnerabilitiesExtension
	GetReadmeExtension          = internal.GetReadmeExtension
	GetForgeExtension           = internal.GetForgeExtension
	GetReleaseExtension         = internal.GetReleaseExtension
	GetCodeOwnersExtension      = internal.GetCodeOwnersExtension
	GetCodeOfConductExtension   = internal.GetCodeOfConductExtension
	GetCitationExtension        = internal.GetCitationExtension

	DisplayName            = internal.DisplayName
	ExtractLocalizedString = internal.ExtractLocalizedString
	SetLocalizedEN         = internal.SetLocalizedEN
	FlatPersonName         = internal.FlatPersonName
	AsStringList           = internal.AsStringList

	LinkURL     = internal.LinkURL
	LinksByType = internal.LinksByType
	SetLink     = internal.SetLink

	PrimaryRepository       = internal.PrimaryRepository
	EnsurePrimaryRepository = internal.EnsurePrimaryRepository
	CitableRepositoryURL    = internal.CitableRepositoryURL
	EnsureCitableSourceLink = internal.EnsureCitableSourceLink

	ContactEmail             = internal.ContactEmail
	ConventionsStyleGuideURL = internal.ConventionsStyleGuideURL
	CopyrightHolders         = internal.CopyrightHolders
	CopyrightHolderNames     = internal.CopyrightHolderNames

	ForgePersonHandle = internal.ForgePersonHandle
	ForgeOrgHandle    = internal.ForgeOrgHandle
)

// Link-type, role, and extension-namespace string constants.
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

	RoleSecurity  = internal.RoleSecurity
	RoleCopyright = internal.RoleCopyright
	RoleCommunity = internal.RoleCommunity

	RepositoryRoleOrigin  = internal.RepositoryRoleOrigin
	RepositoryRoleArchive = internal.RepositoryRoleArchive
	RepositoryRoleMirror  = internal.RepositoryRoleMirror

	ForgeExtensionNS           = internal.ForgeExtensionNS
	FundingExtensionNS         = internal.FundingExtensionNS
	SecurityExtensionNS        = internal.SecurityExtensionNS
	IgnoresExtensionNS         = internal.IgnoresExtensionNS
	VulnerabilitiesExtensionNS = internal.VulnerabilitiesExtensionNS
	ReleaseExtensionNS         = internal.ReleaseExtensionNS
	EditorsExtensionNS         = internal.EditorsExtensionNS
	CodeOwnersExtensionNS      = internal.CodeOwnersExtensionNS
	ContributingExtensionNS    = internal.ContributingExtensionNS
)

// --- Phase 2 additions ---
// Symbols the command layer (the pf-bridge root + forge/scan commands) needs
// beyond the trio's Phase 1 surface. Kept curated to real consumers.

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
	// IssuesRepository — the derive/forges pass resolves the tracker repo.
	IssuesRepository = internal.IssuesRepository
)

// Symbols the moved derive pass reaches (derive was never flipped in Phase 1).
type (
	CLIExtension     = internal.CLIExtension
	CLIDeriveToggles = internal.CLIDeriveToggles
)

var (
	GetCLIExtension = internal.GetCLIExtension
	SetCLIExtension = internal.SetCLIExtension
	AddLink         = internal.AddLink
	LinkByType      = internal.LinkByType
)

// --- Phase 8 additions ---
// Symbols the projectfile CLI (get/set/add/del/convert/optimize/cache) needs
// beyond the trio + pf-bridge surface, now that `internal/cmd` moved to the
// sibling `cli` module and consumes core as a library. Curated to cmd's use.

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

	// YAMLOutputSortedEnabled reads the sorted-output toggle across the boundary
	// (the write is SetYAMLOutputSorted above — a mutable var cannot value-alias).
	YAMLOutputSortedEnabled = internal.YAMLOutputSortedEnabled
)
