import * as rpc from "vlens/rpc"

export type StatedRelation = number;
export const StatedNone: StatedRelation = 0;
export const StatedChild: StatedRelation = 1;
export const StatedParent: StatedRelation = 2;
export const StatedSibling: StatedRelation = 3;
export const StatedPartner: StatedRelation = 4;

export type AccessLevel = number;
export const AccessNone: AccessLevel = 0;
export const AccessView: AccessLevel = 1;
export const AccessContribute: AccessLevel = 2;
export const AccessAdmin: AccessLevel = 3;

export type LinkStatus = number;
export const LinkPending: LinkStatus = 0;
export const LinkAccepted: LinkStatus = 1;
export const LinkRevoked: LinkStatus = 2;

export type GenderType = number;
export const Male: GenderType = 0;
export const Female: GenderType = 1;
export const Unknown: GenderType = 2;

export type MeasurementType = number;
export const Height: MeasurementType = 0;
export const Weight: MeasurementType = 1;

export type RelationKind = number;
export const RelationParent: RelationKind = 0;
export const RelationSibling: RelationKind = 1;
export const RelationPartner: RelationKind = 2;

// Errors
export const ErrLinkNotFound = "Family link not found";
export const ErrLinkToSelf = "A family cannot be linked to itself";
export const ErrLinkExists = "These families are already linked in that direction";
export const ErrTooManyPhotos = "That is more photos than one record can hold";
export const ErrCannotRemoveHomeRoster = "Cannot remove a person from their home family";
export const ErrFaceAnalysisUnavailable = "Face analysis is not available on this server";
export const ErrPhotoWorkerUnavailable = "Photo processing is not running on this server";
export const ErrAdminRequired = "Unauthorized: Admin access required";
export const ErrUserNotFound = "No such user";
export const ErrFamilyAccessDenied = "Access denied: record belongs to another family";
export const ErrNoFamily = "User is not part of a family";
export const ErrPersonNotFound = "Person not found or not in your family";
export const ErrLoginFailure = "LoginFailure";
export const ErrAuthFailure = "AuthFailure";
export const ErrRelationToSelf = "A person cannot be related to themselves";
export const ErrMailNotConfigured = "email delivery is not configured";

export interface CreateAccountRequest {
    name: string
    email: string
    password: string
    confirmPassword: string
    familyCode: string
    initialPersonName: string
    initialPersonGender: number
    initialPersonBirthdate: string
}

export interface CreateAccountResponse {
    success: boolean
    error: string
    token: string
    auth: AuthResponse
}

export interface Empty {
}

export interface AuthResponse {
    id: number
    name: string
    email: string
    isAdmin: boolean
    emailVerified: boolean
    familyId: number
    personId: number
    families: FamilyRef[]
}

export interface FamilyInfoResponse {
    id: number
    name: string
    inviteCode: string
    families: FamilyInfo[]
}

export interface JoinFamilyRequest {
    inviteCode: string
}

export interface JoinFamilyResponse {
    success: boolean
    error: string
    auth: AuthResponse
}

export interface ListFamilyMembersRequest {
    familyId: number
}

export interface ListFamilyMembersResponse {
    familyId: number
    members: FamilyMemberView[]
    callerIsOwner: boolean
}

export interface FamilyIdRequest {
    familyId: number
}

export interface LeaveFamilyResponse {
    success: boolean
    error: string
    auth: AuthResponse
}

export interface RemoveFamilyMemberRequest {
    familyId: number
    userId: number
}

export interface RemoveFamilyMemberResponse {
    success: boolean
    error: string
    members: FamilyMemberView[]
}

export interface RotateInviteCodeResponse {
    success: boolean
    error: string
    familyId: number
    inviteCode: string
}

export interface RequestPasswordResetRequest {
    email: string
}

export interface RequestPasswordResetResponse {
    success: boolean
    error: string
}

export interface ValidatePasswordResetTokenRequest {
    token: string
}

export interface ValidatePasswordResetTokenResponse {
    valid: boolean
}

export interface ResetPasswordRequest {
    token: string
    password: string
    confirmPassword: string
}

export interface ResetPasswordResponse {
    success: boolean
    error: string
}

export interface ListFamilyLinksRequest {
    familyId: number
}

export interface ListFamilyLinksResponse {
    links: FamilyLinkView[]
}

export interface CreateFamilyLinkRequest {
    familyId: number
    inviteCode: string
    kind: string
    scopes: LinkScopes
}

export interface CreateFamilyLinkResponse {
    success: boolean
    error: string
    link: FamilyLinkView
}

export interface FamilyLinkIdRequest {
    id: number
}

export interface FamilyLinkActionResponse {
    success: boolean
    error: string
    link: FamilyLinkView
}

export interface UpdateFamilyLinkRequest {
    id: number
    kind: string
    scopes: LinkScopes
}

export interface GetPersonSharingRequest {
    personId: number
}

export interface GetPersonSharingResponse {
    personId: number
    homeFamilyId: number
    sharedWith: SharedRosterRef[]
    canShare: ShareTargetRef[]
    manageable: boolean
}

export interface SharePersonRequest {
    personId: number
    familyId: number
    relationship: string
}

export interface PersonSharingActionResponse {
    success: boolean
    error: string
    sharing: GetPersonSharingResponse
}

export interface UnsharePersonRequest {
    personId: number
    familyId: number
}

export interface AddPersonRequest {
    name: string
    gender: number
    birthdate: string
    isPregnancy: boolean
    familyId: number
    stated: StatedRelation
    anchorId: number
    additionalAnchorIds: number[]
}

export interface GetPersonResponse {
    person: Person
    growthData: GrowthData[]
    milestones: Milestone[]
    photos: Image[]
}

export interface ListPeopleResponse {
    people: Person[]
    relations: Relation[]
}

export interface GetPersonRequest {
    id: number
}

export interface ComparePeopleRequest {
    personIds: number[]
}

export interface ComparePeopleResponse {
    people: PersonComparisonData[]
}

export interface UpdatePersonRequest {
    id: number
    name: string
    gender: number
    birthdate: string
    isPregnancy: boolean
}

export interface SetProfilePhotoRequest {
    personId: number
    photoId: number
    cropX: number
    cropY: number
    cropScale: number
}

export interface SetProfilePhotoResponse {
    person: Person
}

export interface MergePeopleRequest {
    sourcePersonId: number
    targetPersonId: number
}

export interface MergePeopleResponse {
    success: boolean
    targetPerson: Person
    mergedGrowthCount: number
    mergedMilestones: number
    mergedPhotos: number
}

export interface GetFamilyTimelineRequest {
}

export interface GetFamilyTimelineResponse {
    people: FamilyTimelineItem[]
}

export interface GetPersonRelationsRequest {
    personId: number
}

export interface GetPersonRelationsResponse {
    personId: number
    relations: RelationView[]
    manageable: boolean
}

export interface GetRelationLabelsRequest {
    subjectId: number
}

export interface GetRelationLabelsResponse {
    subjectId: number
    labels: RelationLabelEntry[]
}

export interface AddRelationRequest {
    personId: number
    anchorId: number
    stated: StatedRelation
    additionalAnchorIds: number[]
}

export interface RelationActionResponse {
    success: boolean
    error: string
    relations: GetPersonRelationsResponse
}

export interface RemoveRelationRequest {
    relationId: number
}

export interface AddGrowthDataRequest {
    personId: number
    measurementType: string
    value: number
    unit: string
    inputType: string
    measurementDate: string | null
    ageYears: number | null
    ageMonths: number | null
}

export interface AddGrowthDataResponse {
    growthData: GrowthData
}

export interface GetGrowthDataRequest {
    id: number
}

export interface GetGrowthDataResponse {
    growthData: GrowthData
}

export interface UpdateGrowthDataRequest {
    id: number
    measurementType: string
    value: number
    unit: string
    inputType: string
    measurementDate: string | null
    ageYears: number | null
    ageMonths: number | null
}

export interface UpdateGrowthDataResponse {
    growthData: GrowthData
}

export interface DeleteGrowthDataRequest {
    id: number
}

export interface DeleteGrowthDataResponse {
    success: boolean
}

export interface AddMilestoneRequest {
    personId: number
    description: string
    category: string
    inputType: string
    milestoneDate: string | null
    ageYears: number | null
    ageMonths: number | null
    photoIds: number[]
}

export interface AddMilestoneResponse {
    milestone: Milestone
}

export interface GetPersonMilestonesRequest {
    personId: number
}

export interface GetPersonMilestonesResponse {
    milestones: Milestone[]
}

export interface GetMilestoneRequest {
    id: number
}

export interface GetMilestoneResponse {
    milestone: Milestone
}

export interface UpdateMilestoneRequest {
    id: number
    description: string
    category: string
    inputType: string
    milestoneDate: string | null
    ageYears: number | null
    ageMonths: number | null
    photoIds: number[]
}

export interface UpdateMilestoneResponse {
    milestone: Milestone
}

export interface DeleteMilestoneRequest {
    id: number
}

export interface DeleteMilestoneResponse {
    success: boolean
}

export interface SearchMilestonesRequest {
    query: string
    limit: number | null
}

export interface SearchMilestonesResponse {
    milestones: Milestone[]
    query: string
}

export interface UpdateMilestoneTagsRequest {
    milestoneId: number
    tagIds: number[]
}

export interface UpdateMilestoneTagsResponse {
}

export interface ListActivitiesRequest {
    familyId: number
}

export interface ListActivitiesResponse {
    familyId: number
    activities: Activity[]
}

export interface CreateActivityRequest {
    familyId: number
    name: string
    kind: string
}

export interface ActivityResponse {
    activity: Activity
}

export interface UpdateActivityRequest {
    id: number
    name: string
    kind: string
}

export interface ActivityIdRequest {
    id: number
}

export interface DeleteResponse {
    success: boolean
}

export interface ListSeasonsRequest {
    activityId: number
}

export interface ListSeasonsResponse {
    activityId: number
    seasons: Season[]
}

export interface CreateSeasonRequest {
    activityId: number
    name: string
    startDate: string | null
    endDate: string | null
    notes: string
}

export interface SeasonResponse {
    season: Season
}

export interface UpdateSeasonRequest {
    id: number
    name: string
    startDate: string | null
    endDate: string | null
    notes: string
}

export interface SeasonIdRequest {
    id: number
}

export interface CreateEventRequest {
    seasonId: number
    name: string
    host: string
    location: string
    startDate: string | null
    endDate: string | null
    notes: string
}

export interface EventResponse {
    event: Event
}

export interface UpdateEventRequest {
    id: number
    name: string
    host: string
    location: string
    startDate: string | null
    endDate: string | null
    notes: string
}

export interface EventIdRequest {
    id: number
}

export interface CreateEntryRequest {
    seasonId: number
    name: string
    format: string
    style: string
    division: string
    level: string
    notes: string
    personIds: number[]
}

export interface EntryResponse {
    entry: EntryView
}

export interface UpdateEntryRequest {
    id: number
    name: string
    format: string
    style: string
    division: string
    level: string
    notes: string
}

export interface EntryIdRequest {
    id: number
}

export interface SetEntryRosterRequest {
    entryId: number
    personIds: number[]
}

export interface CreateAppearanceRequest {
    eventId: number
    entryId: number
    occurredAt: string | null
    notes: string
}

export interface AppearanceResponse {
    appearance: AppearanceView
}

export interface UpdateAppearanceRequest {
    id: number
    occurredAt: string | null
    notes: string
}

export interface AppearanceIdRequest {
    id: number
}

export interface SetAppearanceResultsRequest {
    appearanceId: number
    results: ResultInput[]
}

export interface GetSeasonOverviewRequest {
    seasonId: number
}

export interface GetSeasonOverviewResponse {
    activity: Activity
    season: Season
    events: Event[]
    entries: EntryView[]
    appearances: AppearanceView[]
}

export interface GetEventDetailRequest {
    eventId: number
}

export interface GetEventDetailResponse {
    event: Event
    season: SeasonSummary
    photoIds: number[]
    appearances: AppearanceDetail[]
}

export interface GetEntryHistoryRequest {
    entryId: number
}

export interface GetEntryHistoryResponse {
    entry: EntryView
    season: SeasonSummary
    appearances: AppearanceDetail[]
}

export interface GetPersonSeasonRequest {
    personId: number
    seasonId: number
}

export interface GetPersonSeasonResponse {
    personId: number
    seasonId: number
    seasons: SeasonSummary[]
    entries: EntryView[]
    appearances: AppearanceDetail[]
}

export interface ListActivityVocabularyRequest {
    activityId: number
}

export interface ListActivityVocabularyResponse {
    activityId: number
    adjudications: string[]
    awards: string[]
    categories: string[]
    styles: string[]
    divisions: string[]
    levels: string[]
    formats: string[]
    hosts: string[]
}

export interface SetAppearancePhotosRequest {
    appearanceId: number
    photoIds: number[]
}

export interface SetEventPhotosRequest {
    eventId: number
    photoIds: number[]
}

export interface SetEventPhotosResponse {
    eventId: number
    photoIds: number[]
}

export interface CreateTagRequest {
    name: string
    color: string
    familyId: number
}

export interface CreateTagResponse {
    tag: Tag
}

export interface UpdateTagRequest {
    id: number
    name: string
    color: string
}

export interface UpdateTagResponse {
    tag: Tag
}

export interface DeleteTagRequest {
    id: number
}

export interface DeleteTagResponse {
}

export interface ListTagsRequest {
}

export interface ListTagsResponse {
    tags: Tag[]
}

export interface SendMessageRequest {
    content: string
    clientMessageId: string
    familyId: number
}

export interface SendMessageResponse {
    message: ChatMessage
}

export interface GetChatMessagesRequest {
    limit: number | null
    offset: number | null
    familyId: number
}

export interface GetChatMessagesResponse {
    messages: ChatMessage[]
}

export interface DeleteMessageRequest {
    id: number
}

export interface DeleteMessageResponse {
    success: boolean
}

export interface GetPhotoRequest {
    id: number
}

export interface GetPhotoResponse {
    image: Image
    people: Person[]
}

export interface UpdatePhotoRequest {
    id: number
    title: string
    description: string
    inputType: string
    photoDate: string
    ageYears: number | null
    ageMonths: number | null
}

export interface UpdatePhotoResponse {
    image: Image
}

export interface DeletePhotoRequest {
    id: number
}

export interface DeletePhotoResponse {
    success: boolean
}

export interface GetPhotoStatusRequest {
    id: number
}

export interface GetPhotoStatusResponse {
    status: number
}

export interface ListFamilyPhotosRequest {
    personId: number
}

export interface ListFamilyPhotosResponse {
    photos: PhotoWithPeople[]
}

export interface AddPeopleToPhotoRequest {
    photoId: number
    personIds: number[]
}

export interface AddPeopleToPhotoResponse {
    success: boolean
    people: Person[]
}

export interface RemovePersonFromPhotoRequest {
    photoId: number
    personId: number
}

export interface RemovePersonFromPhotoResponse {
    success: boolean
}

export interface UpdatePhotoTagsRequest {
    photoId: number
    tagIds: number[]
}

export interface UpdatePhotoTagsResponse {
}

export interface ImportDataRequest {
    jsonData: string
    filterFamilyIds: number[]
    filterPersonIds: number[]
    previewOnly: boolean
    mergeStrategy: string
    importMilestones: boolean
    importActivities: boolean
    dryRun: boolean
    familyId: number
}

export interface ImportDataResponse {
    importedPeople: number
    mergedPeople: number
    skippedPeople: number
    importedMeasurements: number
    skippedMeasurements: number
    importedMilestones: number
    skippedMilestones: number
    importedTags: number
    skippedTags: number
    importedPhotos: number
    skippedPhotos: number
    importedActivities: ActivityImportCounts
    errors: string[]
    warnings: string[]
    personIdMapping: Record<number, number>
    availableFamilyIds: number[]
    availablePeople: ImportPerson[]
    matchedPeople: PersonMatch[]
}

export interface ExportDataRequest {
    familyId: number
}

export interface ExportDataResponse {
    jsonData: string
}

export interface ListAllUsersResponse {
    users: AdminUserInfo[]
}

export interface GetPhotoStatsRequest {
}

export interface GetPhotoStatsResponse {
    totalPhotos: number
    processedPhotos: number
    pendingPhotos: number
    analysisPending: number
    analysisAnalyzing: number
    analysisDone: number
    analysisFailed: number
    autoTaggedCount: number
    personsWithFace: number
}

export interface ReprocessAllPhotosRequest {
}

export interface ReprocessAllPhotosResponse {
    queued: number
}

export interface ProcessingStats {
    queueLength: number
    isRunning: boolean
    processed: number
    failed: number
    lastProcessedAt: string
    lastError: string
    lastErrorAt: string
    recentAttempts: PhotoAttempt[]
}

export interface AnalysisWorkerStats {
    queueLength: number
    isRunning: boolean
}

export interface ReanalyzeAllPhotosRequest {
}

export interface ReanalyzeAllPhotosResponse {
    queued: number
    skipped: number
}

export interface CheckPhotoConsistencyRequest {
}

export interface PhotoConsistencyReport {
    checkedAt: string
    durationMs: number
    totalImages: number
    presentCount: number
    missingCount: number
    orphanCount: number
    orphanBytes: number
    missing: MissingOriginal[]
    orphans: OrphanOriginal[]
    listLimit: number
    orphanScanErr: string
}

export interface GetLogFilesResponse {
    files: LogFileInfo[]
}

export interface GetLogContentRequest {
    filename: string
    level: string
    category: string
    search: string
    sinceHours: number
    limit: number
    offset: number
    minDuration: number | null
    sortBy: string
    sortDesc: boolean | null
}

export interface GetLogContentResponse {
    entries: PublicLogEntry[]
    totalLines: number
    hasMore: boolean
    filesSearched: string[]
}

export interface LookupLogReferenceRequest {
    reference: string
    context: number
}

export interface LookupLogReferenceResponse {
    found: boolean
    file: string
    entry: PublicLogEntry
    before: PublicLogEntry[]
    after: PublicLogEntry[]
    filesSearched: string[]
}

export interface GetLogStatsResponse {
    stats: LogStats
}

export interface GetPushStatusResponse {
    config: APNsConfigInfo
    stats: PushWorkerStats
    issues: PushConfigIssue[]
    totalDevices: number
    activeDevices: number
    inactiveDevices: number
}

export interface ListPushDevicesResponse {
    devices: AdminPushDevice[]
}

export interface SendTestPushRequest {
    userId: number
    message: string
}

export interface SendTestPushResponse {
    queued: boolean
    deviceCount: number
    targetName: string
}

export interface AnalyticsOverviewResponse {
    totalUsers: number
    totalFamilies: number
    totalPhotos: number
    totalMilestones: number
    activeUsers7d: number
    activeUsers30d: number
    newUsers7d: number
    newUsers30d: number
    recentActivity: ActivitySummary[]
    systemHealth: SystemHealthSummary
}

export interface UserAnalyticsResponse {
    registrationTrends: DataPoint[]
    loginActivityTrends: DataPoint[]
    familySizeDistribution: DistributionPoint[]
    userEngagement: EngagementMetrics
    topActiveFamilies: FamilyActivity[]
}

export interface ContentAnalyticsResponse {
    photoUploadTrends: DataPoint[]
    milestonesByCategory: DistributionPoint[]
    contentPerFamily: FamilyContentStats[]
    photoFormats: DistributionPoint[]
    averagePhotosPerPerson: number
    averageMilestonesPerPerson: number
}

export interface SystemAnalyticsResponse {
    storageUsage: StorageMetrics
    processingMetrics: ProcessingMetrics
    photoFailures: PhotoFailureReport
}

export interface SystemHealthResponse {
    healthy: boolean
    releaseBuild: boolean
    configIssues: ConfigProblem[]
    logs: LogProblems
    photos: PhotoProblems
    push: PushProblems
    mail: MailProblems
    host: HostProblems
    backups: BackupProblems
}

export interface WeeklyDigestResponse {
    since: string
    windowDays: number
    photos: number
    milestones: number
    measurements: number
    messages: number
    people: DigestPerson[]
    accounts: number
    absent: number
    quiet: boolean
}

export interface HostMetricsResponse {
    configured: boolean
    available: boolean
    error: string
    collectedAt: string
    system: HostSystem
    app: HostApp
}

export interface RequeueStuckPhotosRequest {
}

export interface RequeueStuckPhotosResponse {
    queued: number
}

export interface RevokeUserSessionsRequest {
    userId: number
}

export interface RevokeUserSessionsResponse {
    revoked: number
}

export interface VerifyBackupPathRequest {
}

export interface VerifyBackupPathResponse {
    ok: boolean
    detail: string
    status: number
    declaredBytes: number
    receivedBytes: number
    durationMs: number
    checkedAt: string
    cached: boolean
}

export interface GetMailStatsRequest {
}

export interface MailWorkerStats {
    queueLength: number
    isRunning: boolean
    sent: number
    failed: number
    lastSentAt: string
    lastError: string
    lastErrorAt: string
    recentAttempts: MailAttempt[]
}

export interface ResendPasswordResetRequest {
    userId: number
}

export interface ResendPasswordResetResponse {
    email: string
    queued: boolean
    detail: string
    invalidatedPrevious: boolean
    expiresAt: string
}

export interface VerifyEmailRequest {
    token: string
}

export interface VerifyEmailResponse {
    success: boolean
    error: string
}

export interface ResendVerificationResponse {
    success: boolean
    error: string
}

export interface DiagnosticsResponse {
    version: string
    commit: string
    buildTime: string
    release: boolean
    goVersion: string
    startedAt: string
    uptimeSeconds: number
    photoQueue: number
    photoRunning: boolean
    analysisQueue: number
    analysisFaces: boolean
    mailQueue: number
    pushConfigured: boolean
}

export interface RegisterPushDeviceRequest {
    token: string
    platform: string
    environment: string
    bundleId: string
}

export interface RegisterPushDeviceResponse {
    success: boolean
}

export interface UnregisterPushDeviceRequest {
    token: string
}

export interface UnregisterPushDeviceResponse {
    success: boolean
}

export interface NotificationPreferencesResponse {
    chatEnabled: boolean
    showMessageText: boolean
}

export interface UpdateNotificationPreferencesRequest {
    chatEnabled: boolean
    showMessageText: boolean
}

export interface UpdateNotificationPreferencesResponse {
    preferences: NotificationPreferencesResponse
}

export interface CheckMobileVersionRequest {
    platform: string
    appVersion: string
}

export interface CheckMobileVersionResponse {
    status: string
    minimumVersion: string
    latestVersion: string
    updateUrl: string
    updateMessage: string
}

export interface AdminGetMobileVersionsResponse {
    platforms: AdminMobileVersionPlatform[]
}

export interface AdminSetMobileVersionRequest {
    platform: string
    minimumVersion: string
    latestVersion: string
    updateUrl: string
    updateMessage: string
}

export interface AdminSetMobileVersionResponse {
    success: boolean
}

export interface FamilyRef {
    id: number
    name: string
    role: AccessLevel
    isPrimary: boolean
}

export interface FamilyInfo {
    id: number
    name: string
    inviteCode: string
    role: AccessLevel
    isPrimary: boolean
}

export interface FamilyMemberView {
    userId: number
    name: string
    email: string
    role: AccessLevel
    joinedAt: string
    isOwner: boolean
    isSelf: boolean
}

export interface FamilyLinkView {
    id: number
    fromFamilyId: number
    fromFamilyName: string
    toFamilyId: number
    toFamilyName: string
    kind: string
    access: AccessLevel
    scopes: LinkScopes
    status: LinkStatus
    createdAt: string
    outgoing: boolean
    sharedCount: number
}

export interface LinkScopes {
    people: boolean
    milestones: boolean
    photos: boolean
    growth: boolean
    activities: boolean
}

export interface SharedRosterRef {
    familyId: number
    familyName: string
    relationship: string
}

export interface ShareTargetRef {
    familyId: number
    familyName: string
    kind: string
}

export interface Person {
    id: number
    familyId: number
    name: string
    gender: GenderType
    birthday: string
    age: string
    profilePhotoId: number
    profileCropX: number
    profileCropY: number
    profileCropScale: number
    isPregnancy: boolean
    relationship: string
}

export interface GrowthData {
    id: number
    personId: number
    familyId: number
    measurementType: MeasurementType
    value: number
    unit: string
    measurementDate: string
    createdAt: string
}

export interface Milestone {
    id: number
    personId: number
    familyId: number
    description: string
    category: string
    milestoneDate: string
    createdAt: string
    photoIds: number[]
    tagIds: number[]
}

export interface Image {
    id: number
    familyId: number
    ownerUserId: number
    originalFilename: string
    mimeType: string
    fileSize: number
    width: number
    height: number
    filePath: string
    title: string
    description: string
    photoDate: string
    createdAt: string
    status: number
    analysisStatus: number
    tagIds: number[]
}

export interface Relation {
    id: number
    fromId: number
    toId: number
    kind: RelationKind
}

export interface PersonComparisonData {
    person: Person
    growthData: GrowthData[]
    milestones: Milestone[]
    photos: Image[]
}

export interface FamilyTimelineItem {
    person: Person
    growthData: GrowthData[]
    milestones: Milestone[]
    photos: Image[]
}

export interface RelationView {
    id: number
    personId: number
    personName: string
    label: string
    stored: boolean
}

export interface RelationLabelEntry {
    personId: number
    label: string
    group: string
}

export interface Activity {
    id: number
    familyId: number
    name: string
    kind: string
    createdAt: string
}

export interface Season {
    id: number
    activityId: number
    familyId: number
    name: string
    startDate: string
    endDate: string
    notes: string
    createdAt: string
}

export interface Event {
    id: number
    seasonId: number
    familyId: number
    name: string
    host: string
    location: string
    startDate: string
    endDate: string
    notes: string
    createdAt: string
}

export interface EntryView {
    entry: Entry
    personIds: number[]
}

export interface AppearanceView {
    appearance: Appearance
    results: Result[]
    photoIds: number[]
}

export interface ResultInput {
    kind: string
    label: string
    rank: number | null
    outOf: number | null
    category: string
    score: number | null
    personId: number | null
    notes: string
}

export interface SeasonSummary {
    id: number
    name: string
    kind: string
    startDate: string
    endDate: string
}

export interface AppearanceDetail {
    appearance: Appearance
    results: Result[]
    photoIds: number[]
    entry: Entry
    event: EventSummary
}

export interface Tag {
    id: number
    familyId: number
    name: string
    color: string
    createdAt: string
}

export interface ChatMessage {
    id: number
    familyId: number
    userId: number
    userName: string
    content: string
    createdAt: string
    clientMessageId: string
}

export interface PhotoWithPeople {
    image: Image
    people: Person[]
}

export interface ActivityImportCounts {
    activities: number
    seasons: number
    events: number
    entries: number
    appearances: number
    results: number
    reused: number
    skipped: number
}

export interface ImportPerson {
    Id: number
    FamilyId: number
    Type: number
    Gender: number
    Name: string
    Birthday: string
    Age: string
    ImageId: number
}

export interface PersonMatch {
    importPerson: ImportPerson
    existingPerson: Person | null
    matchType: string
    confidence: number
}

export interface AdminUserInfo {
    id: number
    name: string
    email: string
    creation: string
    lastLogin: string
    familyId: number
    familyName: string
    isAdmin: boolean
}

export interface PhotoAttempt {
    time: string
    imageId: number
    reprocess: boolean
    success: boolean
    durationMs: number
    reason: string
}

export interface MissingOriginal {
    imageId: number
    familyId: number
    status: number
    filePath: string
    createdAt: string
}

export interface OrphanOriginal {
    name: string
    sizeBytes: number
    modTime: string
}

export interface LogFileInfo {
    name: string
    size: number
    modTime: string
    isToday: boolean
    sizeString: string
}

export interface PublicLogEntry {
    timestamp: string
    level: string
    category: string
    message: string
    data: any
    userId: number | null
    ip: string
    userAgent: string
    duration: number | null
    handlerDuration: number | null
    httpMethod: string
    httpPath: string
    httpStatus: number | null
    stackTrace: string
}

export interface LogStats {
    totalFiles: number
    totalSize: number
    byLevel: Record<string, number>
    byCategory: Record<string, number>
    recent: PublicLogEntry[]
    errors: PublicLogEntry[]
    performanceStats: PerformanceStats
}

export interface APNsConfigInfo {
    configured: boolean
    teamId: string
    keyId: string
    bundleId: string
    keyPath: string
    environment: string
    keyLoaded: boolean
    loadError: string
}

export interface PushWorkerStats {
    enabled: boolean
    isRunning: boolean
    queueLength: number
    sent: number
    failed: number
    deactivated: number
    suppressed: number
    lastSentAt: string
    lastError: string
    lastErrorAt: string
    recentAttempts: PushAttempt[]
}

export interface PushConfigIssue {
    setting: string
    detail: string
}

export interface AdminPushDevice {
    id: number
    userId: number
    userName: string
    userEmail: string
    tokenHint: string
    platform: string
    environment: string
    bundleId: string
    createdAt: string
    updatedAt: string
    isActive: boolean
    environmentMismatch: boolean
}

export interface ActivitySummary {
    date: string
    photos: number
    milestones: number
    logins: number
}

export interface SystemHealthSummary {
    photosProcessing: number
    photosFailed: number
}

export interface DataPoint {
    date: string
    value: number
}

export interface DistributionPoint {
    label: string
    value: number
}

export interface EngagementMetrics {
    total: number
    neverLoggedIn: number
    active7d: number
    active30d: number
    dormant90d: number
}

export interface FamilyActivity {
    familyName: string
    totalPhotos: number
    totalMilestones: number
    lastActive: string
    score: number
}

export interface FamilyContentStats {
    familyName: string
    photos: number
    milestones: number
    people: number
    photosPerPerson: number
    milestonesPerPerson: number
}

export interface StorageMetrics {
    totalSize: number
    averageFileSize: number
    growthTrend: DataPoint[]
}

export interface ProcessingMetrics {
    successRate: number
    queueLength: number
}

export interface PhotoFailureReport {
    failed: number
    stuck: number
    recentFailures: FailedPhoto[]
}

export interface ConfigProblem {
    setting: string
    detail: string
}

export interface LogProblems {
    windowHours: number
    errors: number
    recentErrors: PublicLogEntry[]
    requests4xx: number
    requests5xx: number
    unavailable: boolean
}

export interface PhotoProblems {
    failed: number
    stuck: number
    analysisFailed: number
    workerStopped: boolean
    queueLength: number
}

export interface PushProblems {
    failed: number
    lastError: string
    lastErrorAt: string
}

export interface MailProblems {
    failed: number
    lastError: string
    lastErrorAt: string
    queueLength: number
}

export interface HostProblems {
    available: boolean
    diskUsedPct: number
    diskLow: boolean
    proxy5xx: number
    proxy4xx: number
    windowSeconds: number
}

export interface BackupProblems {
    available: boolean
    registered: boolean
    neverRun: boolean
    stale: boolean
    lastSuccess: string
    sizeKb: number
}

export interface DigestPerson {
    name: string
    signedIn: boolean
    lastLogin: string
    joined: boolean
    photos: number
    messages: number
}

export interface HostSystem {
    load_avg: HostLoadAvg
    memory: HostMemory
    cpu: HostCPU
    disk: HostDisk
}

export interface HostApp {
    name: string
    disk_kb: number
    traffic: HostTraffic
    backups: HostBackups
    releases: HostRelease[]
}

export interface MailAttempt {
    time: string
    kind: string
    to: string
    success: boolean
    attempts: number
    permanent: boolean
    error: string
}

export interface AdminMobileVersionPlatform {
    platform: string
    configured: boolean
    minimumVersion: string
    latestVersion: string
    updateUrl: string
    updateMessage: string
    allowedHosts: string[]
    warnings: string[]
}

export interface Entry {
    id: number
    seasonId: number
    familyId: number
    name: string
    format: string
    style: string
    division: string
    level: string
    notes: string
    createdAt: string
}

export interface Appearance {
    id: number
    eventId: number
    entryId: number
    familyId: number
    occurredAt: string
    notes: string
    createdAt: string
}

export interface Result {
    id: number
    appearanceId: number
    familyId: number
    kind: string
    label: string
    rank: number | null
    outOf: number | null
    category: string
    score: number | null
    personId: number | null
    notes: string
    sortOrder: number
    createdAt: string
}

export interface EventSummary {
    id: number
    name: string
    host: string
    location: string
    startDate: string
    endDate: string
}

export interface PerformanceStats {
    totalRequests: number
    averageResponse: number
    medianResponse: number
    p90Response: number
    p95Response: number
    p99Response: number
    slowestEndpoints: EndpointStats[]
    endpointStats: Record<string, EndpointStats>
}

export interface PushAttempt {
    time: string
    userId: number
    tokenId: number
    tokenHint: string
    kind: string
    success: boolean
    statusCode: number
    reason: string
    apnsId: string
}

export interface FailedPhoto {
    id: number
    filePath: string
    createdAt: string
}

export interface HostLoadAvg {
    one: number
    five: number
    fifteen: number
}

export interface HostMemory {
    total_kb: number
    available_kb: number
    used_kb: number
    used_pct: number
}

export interface HostCPU {
    user_pct: number
    system_pct: number
    idle_pct: number
    iowait_pct: number
}

export interface HostDisk {
    total_kb: number
    used_kb: number
    free_kb: number
    used_pct: number
}

export interface HostTraffic {
    window_seconds: number
    requests_total: number
    requests_per_min: number
    error_4xx: number
    error_5xx: number
    error_pct: number
}

export interface HostBackups {
    registered: boolean
    last_success: string
    age_seconds: number
    size_kb: number
}

export interface HostRelease {
    name: string
    sha: string
    deployed_at: string
    current: boolean
}

export interface EndpointStats {
    path: string
    method: string
    count: number
    averageResponse: number
    minResponse: number
    maxResponse: number
    errorRate: number
}

export async function CreateAccount(data: CreateAccountRequest): Promise<rpc.Response<CreateAccountResponse>> {
    return await rpc.call<CreateAccountResponse>('CreateAccount', JSON.stringify(data));
}

export async function GetAuthContext(data: Empty): Promise<rpc.Response<AuthResponse>> {
    return await rpc.call<AuthResponse>('GetAuthContext', JSON.stringify(data));
}

export async function GetFamilyInfo(data: Empty): Promise<rpc.Response<FamilyInfoResponse>> {
    return await rpc.call<FamilyInfoResponse>('GetFamilyInfo', JSON.stringify(data));
}

export async function JoinFamily(data: JoinFamilyRequest): Promise<rpc.Response<JoinFamilyResponse>> {
    return await rpc.call<JoinFamilyResponse>('JoinFamily', JSON.stringify(data));
}

export async function ListFamilyMembers(data: ListFamilyMembersRequest): Promise<rpc.Response<ListFamilyMembersResponse>> {
    return await rpc.call<ListFamilyMembersResponse>('ListFamilyMembers', JSON.stringify(data));
}

export async function LeaveFamily(data: FamilyIdRequest): Promise<rpc.Response<LeaveFamilyResponse>> {
    return await rpc.call<LeaveFamilyResponse>('LeaveFamily', JSON.stringify(data));
}

export async function RemoveFamilyMember(data: RemoveFamilyMemberRequest): Promise<rpc.Response<RemoveFamilyMemberResponse>> {
    return await rpc.call<RemoveFamilyMemberResponse>('RemoveFamilyMember', JSON.stringify(data));
}

export async function RotateInviteCode(data: FamilyIdRequest): Promise<rpc.Response<RotateInviteCodeResponse>> {
    return await rpc.call<RotateInviteCodeResponse>('RotateInviteCode', JSON.stringify(data));
}

export async function RequestPasswordReset(data: RequestPasswordResetRequest): Promise<rpc.Response<RequestPasswordResetResponse>> {
    return await rpc.call<RequestPasswordResetResponse>('RequestPasswordReset', JSON.stringify(data));
}

export async function ValidatePasswordResetToken(data: ValidatePasswordResetTokenRequest): Promise<rpc.Response<ValidatePasswordResetTokenResponse>> {
    return await rpc.call<ValidatePasswordResetTokenResponse>('ValidatePasswordResetToken', JSON.stringify(data));
}

export async function ResetPassword(data: ResetPasswordRequest): Promise<rpc.Response<ResetPasswordResponse>> {
    return await rpc.call<ResetPasswordResponse>('ResetPassword', JSON.stringify(data));
}

export async function ListFamilyLinks(data: ListFamilyLinksRequest): Promise<rpc.Response<ListFamilyLinksResponse>> {
    return await rpc.call<ListFamilyLinksResponse>('ListFamilyLinks', JSON.stringify(data));
}

export async function CreateFamilyLink(data: CreateFamilyLinkRequest): Promise<rpc.Response<CreateFamilyLinkResponse>> {
    return await rpc.call<CreateFamilyLinkResponse>('CreateFamilyLink', JSON.stringify(data));
}

export async function AcceptFamilyLink(data: FamilyLinkIdRequest): Promise<rpc.Response<FamilyLinkActionResponse>> {
    return await rpc.call<FamilyLinkActionResponse>('AcceptFamilyLink', JSON.stringify(data));
}

export async function UpdateFamilyLink(data: UpdateFamilyLinkRequest): Promise<rpc.Response<FamilyLinkActionResponse>> {
    return await rpc.call<FamilyLinkActionResponse>('UpdateFamilyLink', JSON.stringify(data));
}

export async function RevokeFamilyLink(data: FamilyLinkIdRequest): Promise<rpc.Response<FamilyLinkActionResponse>> {
    return await rpc.call<FamilyLinkActionResponse>('RevokeFamilyLink', JSON.stringify(data));
}

export async function GetPersonSharing(data: GetPersonSharingRequest): Promise<rpc.Response<GetPersonSharingResponse>> {
    return await rpc.call<GetPersonSharingResponse>('GetPersonSharing', JSON.stringify(data));
}

export async function SharePersonWithFamily(data: SharePersonRequest): Promise<rpc.Response<PersonSharingActionResponse>> {
    return await rpc.call<PersonSharingActionResponse>('SharePersonWithFamily', JSON.stringify(data));
}

export async function UnsharePersonFromFamily(data: UnsharePersonRequest): Promise<rpc.Response<PersonSharingActionResponse>> {
    return await rpc.call<PersonSharingActionResponse>('UnsharePersonFromFamily', JSON.stringify(data));
}

export async function AddPerson(data: AddPersonRequest): Promise<rpc.Response<GetPersonResponse>> {
    return await rpc.call<GetPersonResponse>('AddPerson', JSON.stringify(data));
}

export async function ListPeople(data: Empty): Promise<rpc.Response<ListPeopleResponse>> {
    return await rpc.call<ListPeopleResponse>('ListPeople', JSON.stringify(data));
}

export async function GetPerson(data: GetPersonRequest): Promise<rpc.Response<GetPersonResponse>> {
    return await rpc.call<GetPersonResponse>('GetPerson', JSON.stringify(data));
}

export async function ComparePeople(data: ComparePeopleRequest): Promise<rpc.Response<ComparePeopleResponse>> {
    return await rpc.call<ComparePeopleResponse>('ComparePeople', JSON.stringify(data));
}

export async function UpdatePerson(data: UpdatePersonRequest): Promise<rpc.Response<GetPersonResponse>> {
    return await rpc.call<GetPersonResponse>('UpdatePerson', JSON.stringify(data));
}

export async function SetProfilePhoto(data: SetProfilePhotoRequest): Promise<rpc.Response<SetProfilePhotoResponse>> {
    return await rpc.call<SetProfilePhotoResponse>('SetProfilePhoto', JSON.stringify(data));
}

export async function MergePeople(data: MergePeopleRequest): Promise<rpc.Response<MergePeopleResponse>> {
    return await rpc.call<MergePeopleResponse>('MergePeople', JSON.stringify(data));
}

export async function GetFamilyTimeline(data: GetFamilyTimelineRequest): Promise<rpc.Response<GetFamilyTimelineResponse>> {
    return await rpc.call<GetFamilyTimelineResponse>('GetFamilyTimeline', JSON.stringify(data));
}

export async function GetPersonRelations(data: GetPersonRelationsRequest): Promise<rpc.Response<GetPersonRelationsResponse>> {
    return await rpc.call<GetPersonRelationsResponse>('GetPersonRelations', JSON.stringify(data));
}

export async function GetRelationLabels(data: GetRelationLabelsRequest): Promise<rpc.Response<GetRelationLabelsResponse>> {
    return await rpc.call<GetRelationLabelsResponse>('GetRelationLabels', JSON.stringify(data));
}

export async function AddRelation(data: AddRelationRequest): Promise<rpc.Response<RelationActionResponse>> {
    return await rpc.call<RelationActionResponse>('AddRelation', JSON.stringify(data));
}

export async function RemoveRelation(data: RemoveRelationRequest): Promise<rpc.Response<RelationActionResponse>> {
    return await rpc.call<RelationActionResponse>('RemoveRelation', JSON.stringify(data));
}

export async function AddGrowthData(data: AddGrowthDataRequest): Promise<rpc.Response<AddGrowthDataResponse>> {
    return await rpc.call<AddGrowthDataResponse>('AddGrowthData', JSON.stringify(data));
}

export async function GetGrowthData(data: GetGrowthDataRequest): Promise<rpc.Response<GetGrowthDataResponse>> {
    return await rpc.call<GetGrowthDataResponse>('GetGrowthData', JSON.stringify(data));
}

export async function UpdateGrowthData(data: UpdateGrowthDataRequest): Promise<rpc.Response<UpdateGrowthDataResponse>> {
    return await rpc.call<UpdateGrowthDataResponse>('UpdateGrowthData', JSON.stringify(data));
}

export async function DeleteGrowthData(data: DeleteGrowthDataRequest): Promise<rpc.Response<DeleteGrowthDataResponse>> {
    return await rpc.call<DeleteGrowthDataResponse>('DeleteGrowthData', JSON.stringify(data));
}

export async function AddMilestone(data: AddMilestoneRequest): Promise<rpc.Response<AddMilestoneResponse>> {
    return await rpc.call<AddMilestoneResponse>('AddMilestone', JSON.stringify(data));
}

export async function GetPersonMilestones(data: GetPersonMilestonesRequest): Promise<rpc.Response<GetPersonMilestonesResponse>> {
    return await rpc.call<GetPersonMilestonesResponse>('GetPersonMilestones', JSON.stringify(data));
}

export async function GetMilestone(data: GetMilestoneRequest): Promise<rpc.Response<GetMilestoneResponse>> {
    return await rpc.call<GetMilestoneResponse>('GetMilestone', JSON.stringify(data));
}

export async function UpdateMilestone(data: UpdateMilestoneRequest): Promise<rpc.Response<UpdateMilestoneResponse>> {
    return await rpc.call<UpdateMilestoneResponse>('UpdateMilestone', JSON.stringify(data));
}

export async function DeleteMilestone(data: DeleteMilestoneRequest): Promise<rpc.Response<DeleteMilestoneResponse>> {
    return await rpc.call<DeleteMilestoneResponse>('DeleteMilestone', JSON.stringify(data));
}

export async function SearchMilestones(data: SearchMilestonesRequest): Promise<rpc.Response<SearchMilestonesResponse>> {
    return await rpc.call<SearchMilestonesResponse>('SearchMilestones', JSON.stringify(data));
}

export async function UpdateMilestoneTags(data: UpdateMilestoneTagsRequest): Promise<rpc.Response<UpdateMilestoneTagsResponse>> {
    return await rpc.call<UpdateMilestoneTagsResponse>('UpdateMilestoneTags', JSON.stringify(data));
}

export async function ListActivities(data: ListActivitiesRequest): Promise<rpc.Response<ListActivitiesResponse>> {
    return await rpc.call<ListActivitiesResponse>('ListActivities', JSON.stringify(data));
}

export async function CreateActivity(data: CreateActivityRequest): Promise<rpc.Response<ActivityResponse>> {
    return await rpc.call<ActivityResponse>('CreateActivity', JSON.stringify(data));
}

export async function UpdateActivity(data: UpdateActivityRequest): Promise<rpc.Response<ActivityResponse>> {
    return await rpc.call<ActivityResponse>('UpdateActivity', JSON.stringify(data));
}

export async function DeleteActivity(data: ActivityIdRequest): Promise<rpc.Response<DeleteResponse>> {
    return await rpc.call<DeleteResponse>('DeleteActivity', JSON.stringify(data));
}

export async function ListSeasons(data: ListSeasonsRequest): Promise<rpc.Response<ListSeasonsResponse>> {
    return await rpc.call<ListSeasonsResponse>('ListSeasons', JSON.stringify(data));
}

export async function CreateSeason(data: CreateSeasonRequest): Promise<rpc.Response<SeasonResponse>> {
    return await rpc.call<SeasonResponse>('CreateSeason', JSON.stringify(data));
}

export async function UpdateSeason(data: UpdateSeasonRequest): Promise<rpc.Response<SeasonResponse>> {
    return await rpc.call<SeasonResponse>('UpdateSeason', JSON.stringify(data));
}

export async function DeleteSeason(data: SeasonIdRequest): Promise<rpc.Response<DeleteResponse>> {
    return await rpc.call<DeleteResponse>('DeleteSeason', JSON.stringify(data));
}

export async function CreateEvent(data: CreateEventRequest): Promise<rpc.Response<EventResponse>> {
    return await rpc.call<EventResponse>('CreateEvent', JSON.stringify(data));
}

export async function UpdateEvent(data: UpdateEventRequest): Promise<rpc.Response<EventResponse>> {
    return await rpc.call<EventResponse>('UpdateEvent', JSON.stringify(data));
}

export async function DeleteEvent(data: EventIdRequest): Promise<rpc.Response<DeleteResponse>> {
    return await rpc.call<DeleteResponse>('DeleteEvent', JSON.stringify(data));
}

export async function CreateEntry(data: CreateEntryRequest): Promise<rpc.Response<EntryResponse>> {
    return await rpc.call<EntryResponse>('CreateEntry', JSON.stringify(data));
}

export async function UpdateEntry(data: UpdateEntryRequest): Promise<rpc.Response<EntryResponse>> {
    return await rpc.call<EntryResponse>('UpdateEntry', JSON.stringify(data));
}

export async function DeleteEntry(data: EntryIdRequest): Promise<rpc.Response<DeleteResponse>> {
    return await rpc.call<DeleteResponse>('DeleteEntry', JSON.stringify(data));
}

export async function SetEntryRoster(data: SetEntryRosterRequest): Promise<rpc.Response<EntryResponse>> {
    return await rpc.call<EntryResponse>('SetEntryRoster', JSON.stringify(data));
}

export async function CreateAppearance(data: CreateAppearanceRequest): Promise<rpc.Response<AppearanceResponse>> {
    return await rpc.call<AppearanceResponse>('CreateAppearance', JSON.stringify(data));
}

export async function UpdateAppearance(data: UpdateAppearanceRequest): Promise<rpc.Response<AppearanceResponse>> {
    return await rpc.call<AppearanceResponse>('UpdateAppearance', JSON.stringify(data));
}

export async function DeleteAppearance(data: AppearanceIdRequest): Promise<rpc.Response<DeleteResponse>> {
    return await rpc.call<DeleteResponse>('DeleteAppearance', JSON.stringify(data));
}

export async function SetAppearanceResults(data: SetAppearanceResultsRequest): Promise<rpc.Response<AppearanceResponse>> {
    return await rpc.call<AppearanceResponse>('SetAppearanceResults', JSON.stringify(data));
}

export async function GetSeasonOverview(data: GetSeasonOverviewRequest): Promise<rpc.Response<GetSeasonOverviewResponse>> {
    return await rpc.call<GetSeasonOverviewResponse>('GetSeasonOverview', JSON.stringify(data));
}

export async function GetEventDetail(data: GetEventDetailRequest): Promise<rpc.Response<GetEventDetailResponse>> {
    return await rpc.call<GetEventDetailResponse>('GetEventDetail', JSON.stringify(data));
}

export async function GetEntryHistory(data: GetEntryHistoryRequest): Promise<rpc.Response<GetEntryHistoryResponse>> {
    return await rpc.call<GetEntryHistoryResponse>('GetEntryHistory', JSON.stringify(data));
}

export async function GetPersonSeason(data: GetPersonSeasonRequest): Promise<rpc.Response<GetPersonSeasonResponse>> {
    return await rpc.call<GetPersonSeasonResponse>('GetPersonSeason', JSON.stringify(data));
}

export async function ListActivityVocabulary(data: ListActivityVocabularyRequest): Promise<rpc.Response<ListActivityVocabularyResponse>> {
    return await rpc.call<ListActivityVocabularyResponse>('ListActivityVocabulary', JSON.stringify(data));
}

export async function SetAppearancePhotos(data: SetAppearancePhotosRequest): Promise<rpc.Response<AppearanceResponse>> {
    return await rpc.call<AppearanceResponse>('SetAppearancePhotos', JSON.stringify(data));
}

export async function SetEventPhotos(data: SetEventPhotosRequest): Promise<rpc.Response<SetEventPhotosResponse>> {
    return await rpc.call<SetEventPhotosResponse>('SetEventPhotos', JSON.stringify(data));
}

export async function CreateTag(data: CreateTagRequest): Promise<rpc.Response<CreateTagResponse>> {
    return await rpc.call<CreateTagResponse>('CreateTag', JSON.stringify(data));
}

export async function UpdateTag(data: UpdateTagRequest): Promise<rpc.Response<UpdateTagResponse>> {
    return await rpc.call<UpdateTagResponse>('UpdateTag', JSON.stringify(data));
}

export async function DeleteTag(data: DeleteTagRequest): Promise<rpc.Response<DeleteTagResponse>> {
    return await rpc.call<DeleteTagResponse>('DeleteTag', JSON.stringify(data));
}

export async function ListTags(data: ListTagsRequest): Promise<rpc.Response<ListTagsResponse>> {
    return await rpc.call<ListTagsResponse>('ListTags', JSON.stringify(data));
}

export async function SendMessage(data: SendMessageRequest): Promise<rpc.Response<SendMessageResponse>> {
    return await rpc.call<SendMessageResponse>('SendMessage', JSON.stringify(data));
}

export async function GetChatMessages(data: GetChatMessagesRequest): Promise<rpc.Response<GetChatMessagesResponse>> {
    return await rpc.call<GetChatMessagesResponse>('GetChatMessages', JSON.stringify(data));
}

export async function DeleteMessage(data: DeleteMessageRequest): Promise<rpc.Response<DeleteMessageResponse>> {
    return await rpc.call<DeleteMessageResponse>('DeleteMessage', JSON.stringify(data));
}

export async function GetPhoto(data: GetPhotoRequest): Promise<rpc.Response<GetPhotoResponse>> {
    return await rpc.call<GetPhotoResponse>('GetPhoto', JSON.stringify(data));
}

export async function UpdatePhoto(data: UpdatePhotoRequest): Promise<rpc.Response<UpdatePhotoResponse>> {
    return await rpc.call<UpdatePhotoResponse>('UpdatePhoto', JSON.stringify(data));
}

export async function DeletePhoto(data: DeletePhotoRequest): Promise<rpc.Response<DeletePhotoResponse>> {
    return await rpc.call<DeletePhotoResponse>('DeletePhoto', JSON.stringify(data));
}

export async function GetPhotoStatus(data: GetPhotoStatusRequest): Promise<rpc.Response<GetPhotoStatusResponse>> {
    return await rpc.call<GetPhotoStatusResponse>('GetPhotoStatus', JSON.stringify(data));
}

export async function ListFamilyPhotos(data: ListFamilyPhotosRequest): Promise<rpc.Response<ListFamilyPhotosResponse>> {
    return await rpc.call<ListFamilyPhotosResponse>('ListFamilyPhotos', JSON.stringify(data));
}

export async function AddPeopleToPhoto(data: AddPeopleToPhotoRequest): Promise<rpc.Response<AddPeopleToPhotoResponse>> {
    return await rpc.call<AddPeopleToPhotoResponse>('AddPeopleToPhoto', JSON.stringify(data));
}

export async function RemovePersonFromPhotoProc(data: RemovePersonFromPhotoRequest): Promise<rpc.Response<RemovePersonFromPhotoResponse>> {
    return await rpc.call<RemovePersonFromPhotoResponse>('RemovePersonFromPhotoProc', JSON.stringify(data));
}

export async function UpdatePhotoTags(data: UpdatePhotoTagsRequest): Promise<rpc.Response<UpdatePhotoTagsResponse>> {
    return await rpc.call<UpdatePhotoTagsResponse>('UpdatePhotoTags', JSON.stringify(data));
}

export async function ImportData(data: ImportDataRequest): Promise<rpc.Response<ImportDataResponse>> {
    return await rpc.call<ImportDataResponse>('ImportData', JSON.stringify(data));
}

export async function ExportData(data: ExportDataRequest): Promise<rpc.Response<ExportDataResponse>> {
    return await rpc.call<ExportDataResponse>('ExportData', JSON.stringify(data));
}

export async function ListAllUsers(data: Empty): Promise<rpc.Response<ListAllUsersResponse>> {
    return await rpc.call<ListAllUsersResponse>('ListAllUsers', JSON.stringify(data));
}

export async function GetPhotoStats(data: GetPhotoStatsRequest): Promise<rpc.Response<GetPhotoStatsResponse>> {
    return await rpc.call<GetPhotoStatsResponse>('GetPhotoStats', JSON.stringify(data));
}

export async function ReprocessAllPhotos(data: ReprocessAllPhotosRequest): Promise<rpc.Response<ReprocessAllPhotosResponse>> {
    return await rpc.call<ReprocessAllPhotosResponse>('ReprocessAllPhotos', JSON.stringify(data));
}

export async function GetPhotoProcessingStats(data: Empty): Promise<rpc.Response<ProcessingStats>> {
    return await rpc.call<ProcessingStats>('GetPhotoProcessingStats', JSON.stringify(data));
}

export async function GetAnalysisStats(data: Empty): Promise<rpc.Response<AnalysisWorkerStats>> {
    return await rpc.call<AnalysisWorkerStats>('GetAnalysisStats', JSON.stringify(data));
}

export async function ReanalyzeAllPhotos(data: ReanalyzeAllPhotosRequest): Promise<rpc.Response<ReanalyzeAllPhotosResponse>> {
    return await rpc.call<ReanalyzeAllPhotosResponse>('ReanalyzeAllPhotos', JSON.stringify(data));
}

export async function CheckPhotoConsistency(data: CheckPhotoConsistencyRequest): Promise<rpc.Response<PhotoConsistencyReport>> {
    return await rpc.call<PhotoConsistencyReport>('CheckPhotoConsistency', JSON.stringify(data));
}

export async function GetLogFiles(data: Empty): Promise<rpc.Response<GetLogFilesResponse>> {
    return await rpc.call<GetLogFilesResponse>('GetLogFiles', JSON.stringify(data));
}

export async function GetLogContent(data: GetLogContentRequest): Promise<rpc.Response<GetLogContentResponse>> {
    return await rpc.call<GetLogContentResponse>('GetLogContent', JSON.stringify(data));
}

export async function LookupLogReference(data: LookupLogReferenceRequest): Promise<rpc.Response<LookupLogReferenceResponse>> {
    return await rpc.call<LookupLogReferenceResponse>('LookupLogReference', JSON.stringify(data));
}

export async function GetLogStats(data: Empty): Promise<rpc.Response<GetLogStatsResponse>> {
    return await rpc.call<GetLogStatsResponse>('GetLogStats', JSON.stringify(data));
}

export async function GetPushStatus(data: Empty): Promise<rpc.Response<GetPushStatusResponse>> {
    return await rpc.call<GetPushStatusResponse>('GetPushStatus', JSON.stringify(data));
}

export async function ListPushDevices(data: Empty): Promise<rpc.Response<ListPushDevicesResponse>> {
    return await rpc.call<ListPushDevicesResponse>('ListPushDevices', JSON.stringify(data));
}

export async function SendTestPushNotification(data: SendTestPushRequest): Promise<rpc.Response<SendTestPushResponse>> {
    return await rpc.call<SendTestPushResponse>('SendTestPushNotification', JSON.stringify(data));
}

export async function GetAnalyticsOverview(data: Empty): Promise<rpc.Response<AnalyticsOverviewResponse>> {
    return await rpc.call<AnalyticsOverviewResponse>('GetAnalyticsOverview', JSON.stringify(data));
}

export async function GetUserAnalytics(data: Empty): Promise<rpc.Response<UserAnalyticsResponse>> {
    return await rpc.call<UserAnalyticsResponse>('GetUserAnalytics', JSON.stringify(data));
}

export async function GetContentAnalytics(data: Empty): Promise<rpc.Response<ContentAnalyticsResponse>> {
    return await rpc.call<ContentAnalyticsResponse>('GetContentAnalytics', JSON.stringify(data));
}

export async function GetSystemAnalytics(data: Empty): Promise<rpc.Response<SystemAnalyticsResponse>> {
    return await rpc.call<SystemAnalyticsResponse>('GetSystemAnalytics', JSON.stringify(data));
}

export async function GetSystemHealth(data: Empty): Promise<rpc.Response<SystemHealthResponse>> {
    return await rpc.call<SystemHealthResponse>('GetSystemHealth', JSON.stringify(data));
}

export async function GetWeeklyDigest(data: Empty): Promise<rpc.Response<WeeklyDigestResponse>> {
    return await rpc.call<WeeklyDigestResponse>('GetWeeklyDigest', JSON.stringify(data));
}

export async function GetHostMetrics(data: Empty): Promise<rpc.Response<HostMetricsResponse>> {
    return await rpc.call<HostMetricsResponse>('GetHostMetrics', JSON.stringify(data));
}

export async function RequeueStuckPhotos(data: RequeueStuckPhotosRequest): Promise<rpc.Response<RequeueStuckPhotosResponse>> {
    return await rpc.call<RequeueStuckPhotosResponse>('RequeueStuckPhotos', JSON.stringify(data));
}

export async function RevokeUserSessions(data: RevokeUserSessionsRequest): Promise<rpc.Response<RevokeUserSessionsResponse>> {
    return await rpc.call<RevokeUserSessionsResponse>('RevokeUserSessions', JSON.stringify(data));
}

export async function VerifyBackupPath(data: VerifyBackupPathRequest): Promise<rpc.Response<VerifyBackupPathResponse>> {
    return await rpc.call<VerifyBackupPathResponse>('VerifyBackupPath', JSON.stringify(data));
}

export async function GetMailStats(data: GetMailStatsRequest): Promise<rpc.Response<MailWorkerStats>> {
    return await rpc.call<MailWorkerStats>('GetMailStats', JSON.stringify(data));
}

export async function ResendPasswordReset(data: ResendPasswordResetRequest): Promise<rpc.Response<ResendPasswordResetResponse>> {
    return await rpc.call<ResendPasswordResetResponse>('ResendPasswordReset', JSON.stringify(data));
}

export async function VerifyEmail(data: VerifyEmailRequest): Promise<rpc.Response<VerifyEmailResponse>> {
    return await rpc.call<VerifyEmailResponse>('VerifyEmail', JSON.stringify(data));
}

export async function ResendVerificationEmail(data: Empty): Promise<rpc.Response<ResendVerificationResponse>> {
    return await rpc.call<ResendVerificationResponse>('ResendVerificationEmail', JSON.stringify(data));
}

export async function GetDiagnostics(data: Empty): Promise<rpc.Response<DiagnosticsResponse>> {
    return await rpc.call<DiagnosticsResponse>('GetDiagnostics', JSON.stringify(data));
}

export async function RegisterPushDevice(data: RegisterPushDeviceRequest): Promise<rpc.Response<RegisterPushDeviceResponse>> {
    return await rpc.call<RegisterPushDeviceResponse>('RegisterPushDevice', JSON.stringify(data));
}

export async function UnregisterPushDevice(data: UnregisterPushDeviceRequest): Promise<rpc.Response<UnregisterPushDeviceResponse>> {
    return await rpc.call<UnregisterPushDeviceResponse>('UnregisterPushDevice', JSON.stringify(data));
}

export async function GetNotificationPreferences(data: Empty): Promise<rpc.Response<NotificationPreferencesResponse>> {
    return await rpc.call<NotificationPreferencesResponse>('GetNotificationPreferences', JSON.stringify(data));
}

export async function UpdateNotificationPreferences(data: UpdateNotificationPreferencesRequest): Promise<rpc.Response<UpdateNotificationPreferencesResponse>> {
    return await rpc.call<UpdateNotificationPreferencesResponse>('UpdateNotificationPreferences', JSON.stringify(data));
}

export async function CheckMobileVersion(data: CheckMobileVersionRequest): Promise<rpc.Response<CheckMobileVersionResponse>> {
    return await rpc.call<CheckMobileVersionResponse>('CheckMobileVersion', JSON.stringify(data));
}

export async function AdminGetMobileVersions(data: Empty): Promise<rpc.Response<AdminGetMobileVersionsResponse>> {
    return await rpc.call<AdminGetMobileVersionsResponse>('AdminGetMobileVersions', JSON.stringify(data));
}

export async function AdminSetMobileVersion(data: AdminSetMobileVersionRequest): Promise<rpc.Response<AdminSetMobileVersionResponse>> {
    return await rpc.call<AdminSetMobileVersionResponse>('AdminSetMobileVersion', JSON.stringify(data));
}

