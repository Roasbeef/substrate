// HTTP API client for the Subtrate backend.

import type { APIError, APIResponse } from '@/types/api.js';

// API configuration.
const API_BASE_URL = '/api/v1';

// Custom error class for API errors.
export class ApiError extends Error {
  code: string;
  status: number;
  details: Record<string, unknown> | undefined;

  constructor(
    code: string,
    message: string,
    status: number,
    details?: Record<string, unknown>,
  ) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.details = details;
  }
}

// Request options for API calls.
interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE' | undefined;
  body?: unknown | undefined;
  headers?: Record<string, string> | undefined;
  signal?: AbortSignal | undefined;
}

// Generic fetch wrapper with error handling.
async function request<T>(
  endpoint: string,
  options: RequestOptions = {},
): Promise<T> {
  const { method = 'GET', body, headers = {}, signal } = options;

  const url = `${API_BASE_URL}${endpoint}`;

  const requestHeaders: Record<string, string> = {
    'Content-Type': 'application/json',
    ...headers,
  };

  const fetchOptions: RequestInit = {
    method,
    headers: requestHeaders,
  };

  // Only add body if it exists (avoid undefined in RequestInit).
  if (body !== undefined) {
    fetchOptions.body = JSON.stringify(body);
  }

  // Only add signal if it exists.
  if (signal !== undefined) {
    fetchOptions.signal = signal;
  }

  const response = await fetch(url, fetchOptions);

  // Handle non-JSON responses (like 204 No Content).
  if (response.status === 204) {
    return undefined as T;
  }

  const data: unknown = await response.json();

  // Check for error responses.
  if (!response.ok) {
    const errorData = data as APIError;
    throw new ApiError(
      errorData.error?.code ?? 'unknown_error',
      errorData.error?.message ?? 'An unknown error occurred',
      response.status,
      errorData.error?.details,
    );
  }

  return data as T;
}

// GET request helper.
export function get<T>(endpoint: string, signal?: AbortSignal): Promise<T> {
  const options: RequestOptions = { method: 'GET' };
  if (signal !== undefined) {
    options.signal = signal;
  }
  return request<T>(endpoint, options);
}

// POST request helper.
export function post<T>(
  endpoint: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const options: RequestOptions = { method: 'POST' };
  if (body !== undefined) {
    options.body = body;
  }
  if (signal !== undefined) {
    options.signal = signal;
  }
  return request<T>(endpoint, options);
}

// PATCH request helper.
export function patch<T>(
  endpoint: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  const options: RequestOptions = { method: 'PATCH' };
  if (body !== undefined) {
    options.body = body;
  }
  if (signal !== undefined) {
    options.signal = signal;
  }
  return request<T>(endpoint, options);
}

// DELETE request helper.
export function del<T>(endpoint: string, signal?: AbortSignal): Promise<T> {
  const options: RequestOptions = { method: 'DELETE' };
  if (signal !== undefined) {
    options.signal = signal;
  }
  return request<T>(endpoint, options);
}

// Unwrap APIResponse to get just the data.
export function unwrapResponse<T>(response: APIResponse<T>): T {
  return response.data;
}
