/**
 * ! Do not edit manually
 * * The file has been auto-generated using Orval for Hanzo O11y
 * * regenerate with 'yarn generate:api'
 * Hanzo O11y
 *
 * ONE OPERATION. The users service used to carry twenty-four — invites,
 * passwords, reset tokens, member listing, member editing, roles-on-users — and
 * a generated client is a LIST OF THINGS THE SERVER PROMISES. o11y makes none of
 * those promises now, so a client that still offered them would be offering 404s
 * with type safety.
 */
import { useQuery } from 'react-query';
import type {
	InvalidateOptions,
	QueryClient,
	QueryFunction,
	QueryKey,
	UseQueryOptions,
	UseQueryResult,
} from 'react-query';

import type { GetMyUser200, RenderErrorResponseDTO } from '../o11y.schemas';

import { GeneratedAPIInstance } from '../../../generatedAPIInstance';
import type { ErrorType } from '../../../generatedAPIInstance';

/**
 * This endpoint returns the user I belong to
 * @summary Get my user v2
 */
export const getMyUser = (signal?: AbortSignal) => {
	return GeneratedAPIInstance<GetMyUser200>({
		url: `/v1/o11y/users/me`,
		method: 'GET',
		signal,
	});
};

export const getGetMyUserQueryKey = () => {
	return [`/v1/o11y/users/me`] as const;
};

export const getGetMyUserQueryOptions = <
	TData = Awaited<ReturnType<typeof getMyUser>>,
	TError = ErrorType<RenderErrorResponseDTO>,
>(options?: {
	query?: UseQueryOptions<Awaited<ReturnType<typeof getMyUser>>, TError, TData>;
}) => {
	const { query: queryOptions } = options ?? {};

	const queryKey = queryOptions?.queryKey ?? getGetMyUserQueryKey();

	const queryFn: QueryFunction<Awaited<ReturnType<typeof getMyUser>>> = ({
		signal,
	}) => getMyUser(signal);

	return { queryKey, queryFn, ...queryOptions } as UseQueryOptions<
		Awaited<ReturnType<typeof getMyUser>>,
		TError,
		TData
	> & { queryKey: QueryKey };
};

export type GetMyUserQueryResult = NonNullable<
	Awaited<ReturnType<typeof getMyUser>>
>;
export type GetMyUserQueryError = ErrorType<RenderErrorResponseDTO>;

/**
 * @summary Get my user v2
 */

export function useGetMyUser<
	TData = Awaited<ReturnType<typeof getMyUser>>,
	TError = ErrorType<RenderErrorResponseDTO>,
>(options?: {
	query?: UseQueryOptions<Awaited<ReturnType<typeof getMyUser>>, TError, TData>;
}): UseQueryResult<TData, TError> & { queryKey: QueryKey } {
	const queryOptions = getGetMyUserQueryOptions(options);

	const query = useQuery(queryOptions) as UseQueryResult<TData, TError> & {
		queryKey: QueryKey;
	};

	return { ...query, queryKey: queryOptions.queryKey };
}

/**
 * @summary Get my user v2
 */
export const invalidateGetMyUser = async (
	queryClient: QueryClient,
	options?: InvalidateOptions,
): Promise<QueryClient> => {
	await queryClient.invalidateQueries(
		{ queryKey: getGetMyUserQueryKey() },
		options,
	);

	return queryClient;
};

