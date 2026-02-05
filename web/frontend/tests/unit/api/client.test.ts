// Unit tests for the API client.

import { describe, it, expect, beforeEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '../../mocks/server.js';
import { get, post, patch, del, ApiError, unwrapResponse } from '@/api/client';
import type { APIResponse } from '@/types/api.js';

describe('API client', () => {
  describe('get', () => {
    it('should make GET requests and return data', async () => {
      const response = await get<{ status: string }>('/health');
      expect(response.status).toBe('ok');
    });

    it('should handle 404 errors', async () => {
      server.use(
        http.get('/api/v1/not-found', () => {
          return HttpResponse.json(
            { error: { code: 'not_found', message: 'Not found' } },
            { status: 404 },
          );
        }),
      );

      await expect(get('/not-found')).rejects.toThrow(ApiError);
      await expect(get('/not-found')).rejects.toMatchObject({
        code: 'not_found',
        status: 404,
      });
    });
  });

  describe('post', () => {
    it('should make POST requests with body', async () => {
      server.use(
        http.post('/api/v1/echo', async ({ request }) => {
          const body = await request.json();
          return HttpResponse.json(body);
        }),
      );

      const response = await post<{ test: string }>('/echo', { test: 'value' });
      expect(response.test).toBe('value');
    });

    it('should handle 204 No Content responses', async () => {
      server.use(
        http.post('/api/v1/messages/1/ack', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const response = await post('/messages/1/ack', {});
      expect(response).toBeUndefined();
    });
  });

  describe('patch', () => {
    it('should make PATCH requests', async () => {
      server.use(
        http.patch('/api/v1/items/1', async ({ request }) => {
          const body = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ id: 1, ...body });
        }),
      );

      const response = await patch<{ id: number; name: string }>('/items/1', {
        name: 'updated',
      });
      expect(response.name).toBe('updated');
    });
  });

  describe('del', () => {
    it('should make DELETE requests', async () => {
      server.use(
        http.delete('/api/v1/items/1', () => {
          return new HttpResponse(null, { status: 204 });
        }),
      );

      const response = await del('/items/1');
      expect(response).toBeUndefined();
    });
  });

  describe('error handling', () => {
    it('should create ApiError with correct properties', async () => {
      server.use(
        http.get('/api/v1/error', () => {
          return HttpResponse.json(
            {
              error: {
                code: 'custom_error',
                message: 'Custom error message',
                details: { field: 'value' },
              },
            },
            { status: 400 },
          );
        }),
      );

      try {
        await get('/error');
        expect.fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        const apiError = error as ApiError;
        expect(apiError.code).toBe('custom_error');
        expect(apiError.message).toBe('Custom error message');
        expect(apiError.status).toBe(400);
        expect(apiError.details).toEqual({ field: 'value' });
      }
    });
  });

  describe('unwrapResponse', () => {
    it('should extract data from API response', () => {
      const response: APIResponse<string[]> = {
        data: ['a', 'b', 'c'],
        meta: { total: 3, page: 1, page_size: 10 },
      };

      const data = unwrapResponse(response);
      expect(data).toEqual(['a', 'b', 'c']);
    });
  });
});
