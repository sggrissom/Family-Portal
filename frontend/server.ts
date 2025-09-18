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

export interface AddPersonRequest {
    name: string
    personType: number
    gender: number
    birthdate: string
}

export interface GetPersonResponse {
    person: Person
    growthData: GrowthData[]
}

export interface ListPeopleResponse {
    people: Person[]
}

export interface GetPersonRequest {
    id: number
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

export interface Person {
    id: number
    familyId: number
    name: string
    type: PersonType
    gender: GenderType
    birthday: string
    age: number
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

export async function CreateAccount(data: CreateAccountRequest): Promise<rpc.Response<CreateAccountResponse>> {
    return await rpc.call<CreateAccountResponse>('CreateAccount', JSON.stringify(data));
}

export async function GetAuthContext(data: Empty): Promise<rpc.Response<AuthResponse>> {
    return await rpc.call<AuthResponse>('GetAuthContext', JSON.stringify(data));
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

export async function AddGrowthData(data: AddGrowthDataRequest): Promise<rpc.Response<AddGrowthDataResponse>> {
    return await rpc.call<AddGrowthDataResponse>('AddGrowthData', JSON.stringify(data));
}

