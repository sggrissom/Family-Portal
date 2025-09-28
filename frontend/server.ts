import * as rpc from "vlens/rpc"

export type PersonType = number;
export const Parent: PersonType = 0;
export const Child: PersonType = 1;

export type GenderType = number;
export const Male: GenderType = 0;
export const Female: GenderType = 1;
export const Unknown: GenderType = 2;

export type MeasurementType = number;
export const Height: MeasurementType = 0;
export const Weight: MeasurementType = 1;

// Errors
export const ErrLoginFailure = "LoginFailure";
export const ErrAuthFailure = "AuthFailure";

export interface CreateAccountRequest {
    name: string
    email: string
    password: string
    confirmPassword: string
    familyCode: string
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
    familyId: number
}

export interface FamilyInfoResponse {
    id: number
    name: string
    inviteCode: string
}

export interface JoinFamilyRequest {
    inviteCode: string
}

export interface JoinFamilyResponse {
    success: boolean
    error: string
    auth: AuthResponse
}

export interface AddPersonRequest {
    name: string
    personType: number
    gender: number
    birthdate: string
}

export interface GetPersonResponse {
    person: Person
    growthData: GrowthData[]
    milestones: Milestone[]
    photos: Image[]
}

export interface ListPeopleResponse {
    people: Person[]
}

export interface GetPersonRequest {
    id: number
}

export interface SetProfilePhotoRequest {
    personId: number
    photoId: number
}

export interface SetProfilePhotoResponse {
    person: Person
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

export interface SendMessageRequest {
    content: string
    clientMessageId: string | null
}

export interface SendMessageResponse {
    message: ChatMessage
}

export interface GetChatMessagesRequest {
    limit: number | null
    offset: number | null
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
    person: Person
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

export interface ImportDataRequest {
    jsonData: string
    filterFamilyIds: number[]
    filterPersonIds: number[]
    previewOnly: boolean
}

export interface ImportDataResponse {
    importedPeople: number
    importedMeasurements: number
    errors: string[]
    personIdMapping: Record<number, number>
    availableFamilyIds: number[]
    availablePeople: ImportPerson[]
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
}

export interface ReprocessAllPhotosRequest {
}

export interface ReprocessAllPhotosResponse {
    processed: number
    failed: number
    errors: string[]
    totalTime: string
}

export interface ProcessingStats {
    queueLength: number
    isRunning: boolean
}

export interface GetLogFilesResponse {
    files: LogFileInfo[]
}

export interface GetLogContentRequest {
    filename: string
    level: string
    category: string
    limit: number
    offset: number
}

export interface GetLogContentResponse {
    entries: PublicLogEntry[]
    totalLines: number
    hasMore: boolean
}

export interface GetLogStatsResponse {
    stats: LogStats
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
    userRetention: RetentionMetrics
    topActiveFamilies: FamilyActivity[]
}

export interface ContentAnalyticsResponse {
    photoUploadTrends: DataPoint[]
    milestonesByCategory: DistributionPoint[]
    contentPerFamily: FamilyContentStats[]
    photoFormats: DistributionPoint[]
    averagePhotosPerChild: number
    averageMilestonesPerChild: number
}

export interface SystemAnalyticsResponse {
    storageUsage: StorageMetrics
    processingMetrics: ProcessingMetrics
    errorAnalysis: ErrorAnalysis
    apiRequestTrends: DataPoint[]
}

export interface Person {
    id: number
    familyId: number
    name: string
    type: PersonType
    gender: GenderType
    birthday: string
    age: string
    profilePhotoId: number
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
}

export interface Image {
    id: number
    familyId: number
    personId: number
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
}

export interface ChatMessage {
    id: number
    familyId: number
    userId: number
    userName: string
    content: string
    createdAt: string
    clientMessageId: string | null
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
}

export interface LogStats {
    totalFiles: number
    totalSize: number
    byLevel: Record<string, number>
    byCategory: Record<string, number>
    recent: PublicLogEntry[]
    errors: PublicLogEntry[]
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

export interface RetentionMetrics {
    day1: number
    day7: number
    day30: number
    day90: number
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
    children: number
    photosPerChild: number
    milestonesPerChild: number
}

export interface StorageMetrics {
    totalSize: number
    averageFileSize: number
    growthTrend: DataPoint[]
}

export interface ProcessingMetrics {
    successRate: number
    averageProcessTime: number
    queueLength: number
}

export interface ErrorAnalysis {
    totalErrors: number
    errorsByCategory: DistributionPoint[]
    errorsByLevel: DistributionPoint[]
    recentErrors: string[]
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

export async function AddPerson(data: AddPersonRequest): Promise<rpc.Response<GetPersonResponse>> {
    return await rpc.call<GetPersonResponse>('AddPerson', JSON.stringify(data));
}

export async function ListPeople(data: Empty): Promise<rpc.Response<ListPeopleResponse>> {
    return await rpc.call<ListPeopleResponse>('ListPeople', JSON.stringify(data));
}

export async function GetPerson(data: GetPersonRequest): Promise<rpc.Response<GetPersonResponse>> {
    return await rpc.call<GetPersonResponse>('GetPerson', JSON.stringify(data));
}

export async function SetProfilePhoto(data: SetProfilePhotoRequest): Promise<rpc.Response<SetProfilePhotoResponse>> {
    return await rpc.call<SetProfilePhotoResponse>('SetProfilePhoto', JSON.stringify(data));
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

export async function ImportData(data: ImportDataRequest): Promise<rpc.Response<ImportDataResponse>> {
    return await rpc.call<ImportDataResponse>('ImportData', JSON.stringify(data));
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

export async function GetLogFiles(data: Empty): Promise<rpc.Response<GetLogFilesResponse>> {
    return await rpc.call<GetLogFilesResponse>('GetLogFiles', JSON.stringify(data));
}

export async function GetLogContent(data: GetLogContentRequest): Promise<rpc.Response<GetLogContentResponse>> {
    return await rpc.call<GetLogContentResponse>('GetLogContent', JSON.stringify(data));
}

export async function GetLogStats(data: Empty): Promise<rpc.Response<GetLogStatsResponse>> {
    return await rpc.call<GetLogStatsResponse>('GetLogStats', JSON.stringify(data));
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
