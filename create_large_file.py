import string
import random


def generate_random_string(length):
    characters = string.ascii_letters + string.digits
    random_string = ''.join(random.choice(characters) for _ in range(length))
    return random_string


def main():
	file_size = 1_000_000_000  # 1 GB file
	file_path = f'./large_file_{file_size//1_000_000_000}_gb.txt'

	with open(file_path, 'w+') as file_ptr:
		chunk_size = 10_000_000  # doing chunking to reduce the number of disk access
		for _ in range(file_size//chunk_size):
			chunk = generate_random_string(chunk_size)
			file_ptr.write(chunk)


if __name__ == '__main__':
	main()
