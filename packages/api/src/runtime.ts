// Minimal typed client runtime for the gateway OpenAPI (axios-based).

import axios, {
  AxiosHeaders,
  type AxiosInstance,
  type AxiosRequestConfig,
  type AxiosResponse
} from "axios";

export type Tokens = {
  accessToken: string;
  refreshToken: string;
};

export type TokenStore = {
  get: () => Tokens | null;
  set: (tokens: Tokens | null) => void;
};

export type ClientConfig = {
  baseUrl: string; // e.g. http://localhost:8080
  tokenStore: TokenStore;
  requestId?: () => string;
};

export class ApiClient {
  private baseUrl: string;
  private tokenStore: TokenStore;
  private requestId?: () => string;
  private http: AxiosInstance;
  private raw: AxiosInstance;

  constructor(cfg: ClientConfig) {
    this.baseUrl = cfg.baseUrl.replace(/\/+$/, "");
    this.tokenStore = cfg.tokenStore;
    this.requestId = cfg.requestId;

    this.http = axios.create({ baseURL: this.baseUrl });
    this.raw = axios.create({ baseURL: this.baseUrl });

    this.http.interceptors.request.use((config) => {
      const headers = new AxiosHeaders(config.headers ?? {});
      const tokens = this.tokenStore.get();
      if (tokens?.accessToken) {
        headers.set("Authorization", `Bearer ${tokens.accessToken}`);
      }
      if (this.requestId) {
        headers.set("X-Request-Id", this.requestId());
      }
      config.headers = headers;
      return config;
    });

    this.http.interceptors.response.use(
      (response) => response,
      async (error) => {
        const status = error?.response?.status;
        const original = error?.config as AxiosRequestConfig & { _retry?: boolean };
        if (status === 401 && original && !original._retry) {
          original._retry = true;
          const refreshed = await this.refresh();
          if (refreshed) {
            return this.http.request(original);
          }
        }
        return Promise.reject(error);
      }
    );
  }

  request<T = unknown>(path: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>> {
    return this.http.request<T>({ url: path, ...config });
  }

  async refresh(): Promise<boolean> {
    const tokens = this.tokenStore.get();
    if (!tokens?.refreshToken) return false;

    try {
      const resp = await this.raw.post("/api/v1/auth/refresh", {
        refresh_token: tokens.refreshToken
      });
      const data = resp.data as { access_token: string; refresh_token: string };
      if (!data?.access_token || !data?.refresh_token) {
        this.tokenStore.set(null);
        return false;
      }
      this.tokenStore.set({
        accessToken: data.access_token,
        refreshToken: data.refresh_token
      });
      return true;
    } catch {
      this.tokenStore.set(null);
      return false;
    }
  }
}
