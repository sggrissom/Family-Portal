import * as rpc from "vlens/rpc"

export type PersonType = number;
export const Parent: PersonType = 0;
export const Child: PersonType = 1;

export type GenderType = number;
export const Male: GenderType = 0;
export const Female: GenderType = 1;
export const Unknown: GenderType = 2;

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

export interface AddPersonResponse {
    success: boolean
    error: string
    person: Person
}

export interface ListPeopleResponse {
    success: boolean
    error: string
    people: Person[]
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

export async function CreateAccount(data: CreateAccountRequest): Promise<rpc.Response<CreateAccountResponse>> {
    return await rpc.call<CreateAccountResponse>('CreateAccount', JSON.stringify(data));
}

export async function GetAuthContext(data: Empty): Promise<rpc.Response<AuthResponse>> {
    return await rpc.call<AuthResponse>('GetAuthContext', JSON.stringify(data));
}

export async function AddPerson(data: AddPersonRequest): Promise<rpc.Response<AddPersonResponse>> {
    return await rpc.call<AddPersonResponse>('AddPerson', JSON.stringify(data));
}

export async function ListPeople(data: Empty): Promise<rpc.Response<ListPeopleResponse>> {
    return await rpc.call<ListPeopleResponse>('ListPeople', JSON.stringify(data));
}

