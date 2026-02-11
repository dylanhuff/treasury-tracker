import { useCurrentUser } from '../contexts/CurrentUserContext';

export default function UserDisplay() {
  const { users, currentUser, setCurrentUserById, isLoading, error } = useCurrentUser();

  if (isLoading) {
    return (
      <div className="bg-white rounded-lg shadow p-6 mb-6">
        <p className="text-gray-500">Loading users...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-white rounded-lg shadow p-6 mb-6">
        <p className="text-red-500">Error: {error}</p>
      </div>
    );
  }

  if (!currentUser) {
    return (
      <div className="bg-white rounded-lg shadow p-6 mb-6">
        <p className="text-gray-500">No users available</p>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow p-6 mb-6">
      <h2 className="text-xl font-semibold mb-4">Current User</h2>
      <div className="space-y-2">
        <p><span className="font-medium">Name:</span> {currentUser.name}</p>
        <p><span className="font-medium">Balance:</span> ${currentUser.balance.toLocaleString()}</p>
        <p><span className="font-medium">ID:</span> {currentUser.id}</p>
      </div>

      <div className="mt-4">
        <label className="block text-sm font-medium mb-2">Switch User:</label>
        <select
          value={currentUser.id}
          onChange={(e) => setCurrentUserById(Number(e.target.value))}
          className="w-full p-2 border rounded"
        >
          {users.map((user) => (
            <option key={user.id} value={user.id}>
              {user.name}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
