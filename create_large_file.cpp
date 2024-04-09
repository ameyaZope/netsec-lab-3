#include <fstream>
#include <iostream>
#include <random>
#include <string>

std::string generate_random_string(size_t length) {
  const std::string characters =
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  std::string random_string;
  std::random_device rd;
  std::mt19937 generator(rd());
  std::uniform_int_distribution<> distribution(0, characters.size() - 1);

  for (size_t i = 0; i < length; ++i) {
    random_string += characters[distribution(generator)];
  }

  return random_string;
}

int main() {
  const long long fileSize = 1'000'000'000;  // 1 GB file
  std::string filePath = "./large_file_1_gb.txt";

  std::ofstream filePtr(filePath, std::ios::out);
  if (!filePtr.is_open()) {
    std::cerr << "Failed to open file for writing." << std::endl;
    return 1;
  }

  const int chunkSize =
      10'000'000;  // doing chunking to reduce the number of disk access
  for (int i = 0; i < fileSize / chunkSize; ++i) {
    std::string chunk = generate_random_string(chunkSize);
    filePtr.write(chunk.c_str(), chunk.size());
  }

  filePtr.close();
  return 0;
}
